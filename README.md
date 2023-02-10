# Juniper Route Table Comparison CLI tool

Using two XML routing Table snapshots obtained from a Juniper Device, compare the routes present in both snapshots and display a list of anomalies.

Default is to run this against all Routing Instances using switch -vrf ALL
or specify an indivdual routing instance.

`go run routecompare.go -pre ./pre.xml -post ./post.xml -vrf inet.0`

Results are displayed in a table via the Terminal or File if you using the below example:

`go run routecompare.go -pre ./pre.xml -post ./post.xml -vrf ALL -file-output on`




![Screenshot from 2023-02-02 11-29-32](https://user-images.githubusercontent.com/63735312/216313330-f6614402-a4cd-42f5-bf28-2170cf10444e.png)
