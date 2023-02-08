package main

import (
	"flag"
	"fmt"
	"encoding/xml"
	"os"
	"github.com/olekukonko/tablewriter"
	"strings"
	"io/ioutil"

)


type RouteTable struct {
	XMLName          xml.Name `xml:"rpc-reply"`
	RouteInformation struct {
		RouteTable []struct {
			TableName          string `xml:"table-name"`
			Rt                 []struct {
				RtDestination string `xml:"rt-destination"`
				RtEntry       struct {
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
            var nextHops []string
            for _, nh := range rt.RtEntry.Nh {
                nextHops = append(nextHops, nh.To)
		            }
			var via []string
            for _, nh := range rt.RtEntry.Nh {
                via = append(via, nh.Via)
			}
			
			var nhLocalInterfaces []string
            for _, nh := range rt.RtEntry.Nh {
                nextHops = append(nextHops, nh.To)
                via = append(via, nh.Via)
				nhLocalInterfaces = append(nhLocalInterfaces, nh.NhLocalInterface)
            }
				entries = append(entries, RtDestination{
                Destination: rt.RtDestination,
                NextHop:     nextHops,
				Via: via,
				NhLocalInterface: nhLocalInterfaces,
                TableName:   tableName,
            })
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
func createTable(destinationsrt1 *[]RtDestination,destinationsrt2 *[]RtDestination, action string){
	table := tablewriter.NewWriter(os.Stdout)
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
	if table != nil && action=="PRE"{
		fmt.Println("\n***    Pre Route Table  ***")
		fmt.Println("***    Entries not found in the Post Routing Table Output  ***")
		table.Render()
	}
	if table != nil && action=="POST"{
		fmt.Println("\n***    Post Route Table  ***")
		fmt.Println("***    Entries not found in the Pre Routing Table Output  ***")
		table.Render()
	}   
}

func parseFlags() (string, string, []string, bool) {
	preXMLFile := flag.String("pre", "", "pre XML file")
	postXMLFile := flag.String("post", "", "post XML file")
	rt := flag.String("vrf", "ALL", "list of RoutingTables seperated by a comma or ALL")
	help := flag.Bool("help", false, "display usage")

	flag.Parse()
	routinginstance := strings.Split(*rt, ",")

	if *help {
		flag.PrintDefaults()
		fmt.Println("\nVersion 0.1 - RouteCompare for JunOS devices compares 'show route | display xml' output")
		return "", "", nil, true
	}

	if *preXMLFile == "" || *postXMLFile == "" {
		fmt.Println("Both pre and post XML files are required.")
		flag.PrintDefaults()
		return "", "", nil, true
	}

	return *preXMLFile, *postXMLFile, routinginstance, false
}

func main() {
	preXMLFile, postXMLFile, routinginstance, help := parseFlags()
	if help {
		return
	}

	fmt.Println("Pre XML file:", preXMLFile)
	fmt.Println("Post XML file:", postXMLFile)

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

	createTable(&preDestinations, &postDestinations, "PRE")
	createTable(&preDestinations, &postDestinations, "POST")
}
