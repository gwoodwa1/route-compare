package main

import (
	"flag"
	"fmt"
	"encoding/xml"
	"os"
	"github.com/olekukonko/tablewriter"
	"strings"
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

	var routetable RouteTable
	if err := xml.NewDecoder(xmlFile).Decode(&routetable); err != nil {
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

func main() {
	preXMLFile := flag.String("pre", "", "pre XML file")
	postXMLFile := flag.String("post", "", "post XML file")
    rt := flag.String("vrf", "ALL", "list of RoutingTables seperated by a comma or ALL")
	help := flag.Bool("help", false, "display usage")

	flag.Parse()
	routinginstance := strings.Split(*rt, ",")

	if *help {
		flag.PrintDefaults()
		fmt.Println("\nVersion 0.1 - RouteCompare for JunOS devices compares 'show route | display xml' output")
		return
	}

	if *preXMLFile == "" || *postXMLFile == "" {
		fmt.Println("Both pre and post XML files are required.")
		flag.PrintDefaults()
		return
	}

	fmt.Println("Pre XML file:", *preXMLFile)
	fmt.Println("Post XML file:", *postXMLFile)

	preRpcReply, err := parseXMLFile(*preXMLFile)
	if err != nil {
		fmt.Println("Error parsing pre XML file:", err)
		return
	}

	postRpcReply, postErr := parseXMLFile(*postXMLFile)
	if postErr != nil {
		fmt.Println("Error parsing post XML file:", postErr)
		return
	}

	preDestinations := getRtDestinationEntries(preRpcReply,routinginstance)
	postDestinations := getRtDestinationEntries(postRpcReply,routinginstance)
	
	pretable := tablewriter.NewWriter(os.Stdout)
	pretable.SetHeader([]string{"Destination", "NextHop","Via","NhLocalInterface","Routing-Instance"})

	for _, preDest := range preDestinations {
		found := false
		for _, postDest := range postDestinations {
			if preDest.Destination == postDest.Destination && isSameSlice(preDest.NhLocalInterface,postDest.NhLocalInterface) && isSameSlice(preDest.NextHop, postDest.NextHop) && isSameSlice(preDest.Via, postDest.Via){
				found = true
				break
			}
		}
		if !found {
			pretable.Append([]string{fmt.Sprintf("%v", preDest.Destination), fmt.Sprintf("%v", preDest.NextHop),fmt.Sprintf("%v", preDest.Via), fmt.Sprintf("%v", preDest.NhLocalInterface), fmt.Sprintf("%v", preDest.TableName)})
            
		}
		
	}
    if pretable != nil{
		fmt.Println("***    Pre Route Table  ***")
		fmt.Println("***    Entries not found in the Post Routing Table Output  ***")
		pretable.Render()
	}
	
	posttable := tablewriter.NewWriter(os.Stdout)
	posttable.SetHeader([]string{"Destination", "NextHop","Via","NhLocalInterface","Routing-Instance"})
	
	for _, postDest := range postDestinations {
		found := false
		for _, preDest := range preDestinations {
			if postDest.Destination == preDest.Destination && isSameSlice(postDest.NhLocalInterface,preDest.NhLocalInterface) && isSameSlice(postDest.NextHop,preDest.NextHop) && isSameSlice(postDest.Via,preDest.Via) {
				found = true
				break
			}
		}
		if !found {
			posttable.Append([]string{fmt.Sprintf("%v", postDest.Destination), fmt.Sprintf("%v", postDest.NextHop),fmt.Sprintf("%v", postDest.Via), fmt.Sprintf("%v", postDest.NhLocalInterface), fmt.Sprintf("%v", postDest.TableName)})
		}
	}
	if posttable != nil{
		fmt.Println("\n\n***    Post Route Table  ***")
		fmt.Println("***    Entries not found in the Pre Routing Table Output  ***")
		posttable.Render()
	}
}
	

