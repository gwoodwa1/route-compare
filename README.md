# Juniper Route Table Comparison CLI tool

Using two XML routing Table snapshots obtained from a Juniper Device, compare the routes present in both snapshots and display a list of anomalies.

Default is to run this against all Routing Instances using switch -vrf ALL
or specify an indivdual routing instance.

`go run routecompare.go -pre ./pre.xml -post ./post.xml -vrf inet.0`

Results are displayed in a table via the Terminal



![Screenshot from 2023-01-31 06-52-17](https://user-images.githubusercontent.com/63735312/215687734-c5429319-94d4-4bff-ab33-a45dbb5e1f04.png)
