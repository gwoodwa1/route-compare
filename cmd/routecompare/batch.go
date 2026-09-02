package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type batchManifest struct {
	Comparisons []batchComparison `json:"comparisons"`
}

type batchComparison struct {
	Name       string   `json:"name"`
	Pre        string   `json:"pre"`
	Post       string   `json:"post"`
	Device     string   `json:"device"`
	ChangeID   string   `json:"change_id"`
	Tables     []string `json:"tables"`
	Protocols  []string `json:"protocols"`
	Prefixes   []string `json:"prefixes"`
	ChangeTypes []string `json:"change_types"`
	IgnoreFields []string `json:"ignore_fields"`
	Policy     string   `json:"policy"`
}

type batchReport struct {
	Reports []report `json:"reports"`
}

func runBatch(manifestPath, format, output, failOn string, stdout io.Writer) error {
	manifest, err := loadBatchManifest(manifestPath)
	if err != nil {
		return err
	}
	base := filepath.Dir(manifestPath)
	result := batchReport{Reports: make([]report, 0, len(manifest.Comparisons))}
	failed := false
	for _, comparison := range manifest.Comparisons {
		args := []string{
			"-pre", resolveManifestPath(base, comparison.Pre),
			"-post", resolveManifestPath(base, comparison.Post),
			"-format", "json",
			"-device", firstNonEmpty(comparison.Device, comparison.Name),
		}
		appendListFlag := func(name string, values []string) {
			if len(values) > 0 {
				args = append(args, name, strings.Join(values, ","))
			}
		}
		appendListFlag("-vrf", comparison.Tables)
		appendListFlag("-protocol", comparison.Protocols)
		appendListFlag("-prefix", comparison.Prefixes)
		appendListFlag("-change-type", comparison.ChangeTypes)
		appendListFlag("-ignore", comparison.IgnoreFields)
		if comparison.ChangeID != "" {
			args = append(args, "-change-id", comparison.ChangeID)
		}
		if comparison.Policy != "" {
			args = append(args, "-policy", resolveManifestPath(base, comparison.Policy))
		}
		if failOn != "none" {
			args = append(args, "-fail-on", failOn)
		}
		var jobOutput, jobErrors bytes.Buffer
		err := run(args, &jobOutput, &jobErrors)
		var differenceErr differenceFoundError
		if err != nil && !errors.As(err, &differenceErr) {
			return fmt.Errorf("batch comparison %q: %w", comparison.Name, err)
		}
		var jobReport report
		if decodeErr := json.Unmarshal(jobOutput.Bytes(), &jobReport); decodeErr != nil {
			return fmt.Errorf("batch comparison %q: decode internal report: %w", comparison.Name, decodeErr)
		}
		if jobReport.Failed {
			failed = true
		}
		result.Reports = append(result.Reports, jobReport)
	}

	w := stdout
	var file *os.File
	if output != "" {
		file, err = os.Create(output)
		if err != nil {
			return fmt.Errorf("create batch report %q: %w", output, err)
		}
		w = file
	}
	if err := writeBatchReport(w, strings.ToLower(format), result); err != nil {
		if file != nil {
			_ = file.Close()
		}
		return err
	}
	if file != nil {
		if err := file.Close(); err != nil {
			return fmt.Errorf("close batch report %q: %w", output, err)
		}
	}
	if failed {
		return differenceFoundError{policy: "batch"}
	}
	return nil
}

func loadBatchManifest(path string) (*batchManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open batch manifest %q: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest batchManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse batch manifest %q: %w", path, err)
	}
	if len(manifest.Comparisons) == 0 {
		return nil, fmt.Errorf("parse batch manifest %q: no comparisons found", path)
	}
	seen := make(map[string]struct{})
	for i, comparison := range manifest.Comparisons {
		if comparison.Name == "" || comparison.Pre == "" || comparison.Post == "" {
			return nil, fmt.Errorf("parse batch manifest %q: comparison %d requires name, pre, and post", path, i+1)
		}
		if _, duplicate := seen[comparison.Name]; duplicate {
			return nil, fmt.Errorf("parse batch manifest %q: duplicate comparison name %q", path, comparison.Name)
		}
		seen[comparison.Name] = struct{}{}
	}
	return &manifest, nil
}

func writeBatchReport(w io.Writer, format string, result batchReport) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "text":
		for i, report := range result.Reports {
			if i > 0 {
				fmt.Fprintln(w, "\n============================================================")
			}
			renderText(w, report)
		}
		return nil
	case "markdown":
		fmt.Fprintln(w, "# Batch route comparison report")
		for _, report := range result.Reports {
			fmt.Fprintln(w, "\n---\n")
			renderMarkdown(w, report)
		}
		return nil
	case "junit":
		return renderBatchJUnit(w, result)
	default:
		return fmt.Errorf("batch output does not support format %q (use text, json, markdown, or junit)", format)
	}
}

type junitBatchSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

func renderBatchJUnit(w io.Writer, result batchReport) error {
	suite := junitBatchSuite{Name: "routecompare-batch", Tests: len(result.Reports), Time: "0"}
	for _, report := range result.Reports {
		name := firstNonEmpty(report.Metadata.Device, report.Before.Path+" -> "+report.After.Path)
		details := "before=" + strconv.Itoa(report.Summary.Before) + " after=" + strconv.Itoa(report.Summary.After) + " added=" + strconv.Itoa(report.Summary.Added) + " removed=" + strconv.Itoa(report.Summary.Removed) + " modified=" + strconv.Itoa(report.Summary.Modified)
		testCase := junitTestCase{Name: name, ClassName: "routecompare", SystemOut: details}
		if report.Failed {
			suite.Failures++
			message, body := "route comparison failed", details
			if report.Policy != nil && !report.Policy.Passed {
				message = "policy " + report.Policy.Name + " failed"
				body = strings.Join(report.Policy.Violations, "\n")
			}
			testCase.Failure = &junitFailure{Message: message, Body: body}
		}
		suite.Cases = append(suite.Cases, testCase)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	return encoder.Encode(suite)
}

func resolveManifestPath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
