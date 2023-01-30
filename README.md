# Juniper Route Table Comparison CLI tool

Using two XML routing Table snapshots obtained from a Juniper Device, compare the routes present in both snapshots and display a list of anomalies.

Default is to run this against all Routing Instances using switch -vrf ALL
or specify an indivdual routing instance.

`go run routecompare.go -pre ./pre.xml -post ./post.xml -vrf inet.0`

Results are displayed in a table via the Terminal

![Screenshot from 2023-01-30 13-00-51](https://user-images.githubusercontent.com/63735312/215485142-de005d96-649e-4110-b8b7-019dccc77a4d.png)

