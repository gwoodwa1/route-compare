package main

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"

	routecompare "github.com/gwoodwa1/route-compare"
)

type comparisonPolicy struct {
	Name             string   `json:"name"`
	Tables           []string `json:"tables"`
	Protocols        []string `json:"protocols"`
	Prefixes         []string `json:"prefixes"`
	ChangeTypes      []string `json:"change_types"`
	IgnoreFields     []string `json:"ignore_fields"`
	FailOn           string   `json:"fail_on"`
	MaxAdded         *int     `json:"max_added"`
	MaxRemoved       *int     `json:"max_removed"`
	MaxModified      *int     `json:"max_modified"`
	CriticalPrefixes []string `json:"critical_prefixes"`
}

type policyEvaluation struct {
	Name       string   `json:"name"`
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations"`
}

func loadPolicy(path string) (*comparisonPolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open policy %q: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var policy comparisonPolicy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("parse policy %q: %w", path, err)
	}
	if strings.TrimSpace(policy.Name) == "" {
		return nil, fmt.Errorf("parse policy %q: name is required", path)
	}
	for name, threshold := range map[string]*int{
		"max_added": policy.MaxAdded, "max_removed": policy.MaxRemoved, "max_modified": policy.MaxModified,
	} {
		if threshold != nil && *threshold < 0 {
			return nil, fmt.Errorf("parse policy %q: %s cannot be negative", path, name)
		}
	}
	if policy.FailOn != "" && !validFailPolicy(strings.ToLower(policy.FailOn)) {
		return nil, fmt.Errorf("parse policy %q: unsupported fail_on %q", path, policy.FailOn)
	}
	if err := validateChangeTypes(normalizeLower(policy.ChangeTypes)); err != nil {
		return nil, fmt.Errorf("parse policy %q: %w", path, err)
	}
	if err := validateIgnoreFields(policy.IgnoreFields); err != nil {
		return nil, fmt.Errorf("parse policy %q: %w", path, err)
	}
	if _, _, err := parsePrefixList(policy.Prefixes); err != nil {
		return nil, fmt.Errorf("parse policy %q prefixes: %w", path, err)
	}
	if _, _, err := parsePrefixList(policy.CriticalPrefixes); err != nil {
		return nil, fmt.Errorf("parse policy %q critical_prefixes: %w", path, err)
	}
	return &policy, nil
}

func validateIgnoreFields(fields []string) error {
	supported := map[string]struct{}{
		"protocol": {}, "preference": {}, "next_hop_type": {}, "next_hops": {},
		"active": {}, "hidden": {}, "metric": {}, "metric2": {},
		"local_preference": {}, "as_path": {}, "communities": {}, "tag": {},
	}
	for _, field := range fields {
		if _, ok := supported[strings.ToLower(strings.TrimSpace(field))]; !ok {
			return fmt.Errorf("unsupported ignored field %q", field)
		}
	}
	return nil
}

func evaluatePolicy(policy *comparisonPolicy, diff routecompare.Difference) (*policyEvaluation, error) {
	if policy == nil {
		return nil, nil
	}
	result := &policyEvaluation{Name: policy.Name, Passed: true, Violations: []string{}}
	checkMaximum := func(name string, maximum *int, actual int) {
		if maximum != nil && actual > *maximum {
			result.Violations = append(result.Violations, fmt.Sprintf("%s routes: %d exceeds maximum %d", name, actual, *maximum))
		}
	}
	checkMaximum("added", policy.MaxAdded, len(diff.Added))
	checkMaximum("removed", policy.MaxRemoved, len(diff.Removed))
	checkMaximum("modified", policy.MaxModified, len(diff.Modified))

	critical, _, err := parsePrefixList(policy.CriticalPrefixes)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	checkCritical := func(changeType string, route routecompare.Route) {
		if len(critical) == 0 || !matchesPrefix(route.Destination, critical) {
			return
		}
		key := changeType + "\x00" + route.Table + "\x00" + route.Destination
		if _, duplicate := seen[key]; duplicate {
			return
		}
		seen[key] = struct{}{}
		result.Violations = append(result.Violations, fmt.Sprintf("critical prefix %s in %s was %s", route.Destination, route.Table, changeType))
	}
	for _, route := range diff.Added {
		checkCritical("added", route)
	}
	for _, route := range diff.Removed {
		checkCritical("removed", route)
	}
	for _, change := range diff.Modified {
		checkCritical("modified", change.Before)
	}
	result.Passed = len(result.Violations) == 0
	return result, nil
}

func parsePrefixList(names []string) ([]netip.Prefix, []string, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}
	return parsePrefixes(strings.Join(names, ","))
}

func normalizeLower(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}
