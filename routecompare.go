package main

import (
	"flag"
	"fmt"
	"encoding/xml"
	"os"
	"github.com/olekukonko/tablewriter"
	"strings"
	"io/ioutil"
	"io"

)


type RouteTable struct {
	XMLName          xml.Name `xml:"rpc-reply"`
	RouteInformation struct {
		RouteTable []struct {
			TableName          string `xml:"table-name"`
			Rt                 []struct {
				RtDestination string `xml:"rt-destination"`
				RtEntry       []struct {
					ProtocolName  string `xml:"protocol-name"`
					Preference    string `xml:"preference"`
					Age           struct {
						Text    string `xml:",chardata"`
						Seconds string `xml:"seconds,attr"`
					} `xml:"age"`
					Nh []struct {
						Text             string `xml:",chardata"`
						SelectedNextHop  string `xml:"selected-next-hop"`
						To               string `xml:"to"`
						Via              string `xml:"via"`
						NhLocalInterface string `xml:"nh-local-interface"`
					} `xml:"nh"`
					NhType string `xml:"nh-type"`
				} `xml:"rt-entry"`
			} `xml:"rt"`
		} `xml:"route-table"`
	} `xml:"route-information"`
} 

// parseXMLFile parses an XML file and returns a RouteTable and an error.
//
// fileName is the name of the XML file to be parsed.
//
// If the XML file can be successfully parsed, a pointer to a RouteTable and a nil error
// are returned. Otherwise, a nil pointer to a RouteTable and an error are returned.

func parseXMLFile(fileName string) (*RouteTable, error) {
    xmlFile, err := os.Open(fileName)
    if err != nil {
        return nil, err
    }
    defer xmlFile.Close()

    xmlData, err := ioutil.ReadAll(xmlFile)
    if err != nil {
        return nil, err
    }
    // Remove all that Whitespace which is taking up memory
    xmlData = []byte(strings.Replace(string(xmlData), "  ", "", -1))
    xmlData = []byte(strings.Replace(string(xmlData), "\n", "", -1))

    var routetable RouteTable
    if err := xml.Unmarshal(xmlData, &routetable); err != nil {
        return nil, err
    }

    return &routetable, nil
}



// getRtDestinationEntries retrieves the RtDestination entries for the specified routing instances.
//
// reply is a pointer to a RouteTable.
// routinginstance is a list of routing instances to retrieve entries from. If routinginstance is ["ALL"],
// entries from all routing instances are returned.
//
// The function returns a list of RtDestination entries.


func getRtDestinationEntries(reply *RouteTable, routinginstance []string) []RtDestination {
    var entries []RtDestination
    for _, routeTable := range reply.RouteInformation.RouteTable {
        tableName := routeTable.TableName
        if !contains(routinginstance, "ALL") && !contains(routinginstance, tableName) {
            continue
        }
        for _, rt := range routeTable.Rt {
            for _, rtEntry := range rt.RtEntry {
                var nextHops []string
                var via []string
                var nhLocalInterfaces []string
                for _, nh := range rtEntry.Nh {
                    nextHops = append(nextHops, nh.To)
                    via = append(via, nh.Via)
                    nhLocalInterfaces = append(nhLocalInterfaces, nh.NhLocalInterface)
                }
                entries = append(entries, RtDestination{
                    Destination: rt.RtDestination,
                    NextHop:     nextHops,
                    Via:         via,
                    NhLocalInterface: nhLocalInterfaces,
                    TableName:   tableName,
                })
            }
        }
    }
    return entries
}


// contains checks if a string is in a list of strings.
//
// s is a list of strings.
// e is a string to be checked for in s.
//
// The function returns a boolean indicating whether e is in s.

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

type RtDestination struct {
	Destination string
	NextHop     []string
	Via         []string
	TableName   string
	NhLocalInterface []string
}

// Function to check if two slices are identical

func isSameSlice(a, b []string) bool {
    if len(a) != len(b) {
        return false
    }
    for i, v := range a {
        if v != b[i] {
            return false
        }
    }
    return true
}


// Function to create the Tables displaying the differences in the Routing Tables
func createTable(destinationsrt1 *[]RtDestination,destinationsrt2 *[]RtDestination, action string,outputfile string)(table *tablewriter.Table){
	
	var file io.Writer
	var fileName string

	if outputfile != "off"{
		fileName = fmt.Sprintf("%s.txt", action)
		f, err := os.Create(fileName)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		defer f.Close()
		file = f
	} else {
		file = os.Stdout
	}

	table = tablewriter.NewWriter(file)
	table.SetHeader([]string{"Destination", "NextHop","Via","NhLocalInterface","Routing-Instance"})
	for _, rt1Dest := range *destinationsrt1 {
		found := false
		for _, rt2Dest := range *destinationsrt2{
			if rt1Dest.Destination == rt2Dest.Destination && isSameSlice(rt1Dest.NhLocalInterface,rt2Dest.NhLocalInterface) && isSameSlice(rt1Dest.NextHop, rt2Dest.NextHop) && isSameSlice(rt1Dest.Via, rt2Dest.Via){
				found = true
				break
			}
		}
		if !found {
			table.Append([]string{fmt.Sprintf("%v", rt1Dest.Destination), fmt.Sprintf("%v", rt1Dest.NextHop),fmt.Sprintf("%v", rt1Dest.Via), fmt.Sprintf("%v", rt1Dest.NhLocalInterface), fmt.Sprintf("%v", rt1Dest.TableName)})
            
		}
		
	}
	if table != nil && action=="PRE" && outputfile == "off"{
		fmt.Println("\n***    Pre Route Table  ***")
		fmt.Println("***    Entries not found in the Post Routing Table Output  ***")

	}
	if table != nil && action=="POST" && outputfile == "off"{
		fmt.Println("\n***    Post Route Table  ***")
		fmt.Println("***    Entries not found in the Pre Routing Table Output  ***")

	}else if outputfile != "off" {
		fmt.Printf("\n***    Output to File - %s ***\n",fileName)
	}
	table.Render()
	return table 
}





func parseFlags() (string, string, string, []string, bool) {
	preXMLFile := flag.String("pre", "", "pre XML file")
	postXMLFile := flag.String("post", "", "post XML file")
	outputFile := flag.String("file-output", "off", "on or off to write table to file. default off")
	rt := flag.String("vrf", "ALL", "list of RoutingTables seperated by a comma or ALL")
	help := flag.Bool("help", false, "display usage")

	flag.Parse()
	routinginstance := strings.Split(*rt, ",")

	if *help {
		flag.PrintDefaults()
		fmt.Println("\nVersion 0.1 - RouteCompare for JunOS devices compares 'show route | display xml' output")
		return "", "", "",nil, true
	}

	if *preXMLFile == "" || *postXMLFile == "" {
		fmt.Println("Both pre and post XML files are required.")
		flag.PrintDefaults()
		return "", "", "",nil,true
	}

	return *preXMLFile, *postXMLFile,*outputFile,routinginstance,false
}

func main() {
	// Deals with the command line options and passes them to the parseFlags() function
	preXMLFile, postXMLFile,outputFile,routinginstance, help := parseFlags()
	if help {
		return
	}

	fmt.Println("Pre XML file:", preXMLFile)
	fmt.Println("Post XML file:", postXMLFile)

	// Create a channel to pass the pre and post XML files
	results := make(chan *RouteTable, 2)

	go func() {
		preRpcReply, err := parseXMLFile(preXMLFile)
		if err != nil {
			fmt.Println("Error parsing pre XML file:", err)
			return
		}
		results <- preRpcReply
	}()

	go func() {
		postRpcReply, postErr := parseXMLFile(postXMLFile)
		if postErr != nil {
			fmt.Println("Error parsing post XML file:", postErr)
			return
		}
		results <- postRpcReply
	}()

	preRpcReply := <-results
	postRpcReply := <-results

	// Create a channel to pass the Destination Prefixes and Entries

	destinationCh := make(chan []RtDestination, 2)
	
	go func() {
		preDestinations := getRtDestinationEntries(preRpcReply, routinginstance)
		destinationCh <- preDestinations
	}()

	go func() {
		postDestinations := getRtDestinationEntries(postRpcReply, routinginstance)
		destinationCh <- postDestinations
	}()

	preDestinations := <-destinationCh
	postDestinations := <-destinationCh

	// This block we deal with rendering the table to the terminal or file using a channel to run concurrent calls
	
	tableCh := make(chan *tablewriter.Table, 2)

	go func() {
		tableCh <- createTable(&preDestinations, &postDestinations, "PRE", outputFile)
	}()
	
	go func() {
		tableCh <- createTable(&preDestinations, &postDestinations, "POST", outputFile)
	}()
	
	<-tableCh
	<-tableCh

}

	



