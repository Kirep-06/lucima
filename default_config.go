package lucima

var DefaultConfig = []byte(`
{
	"DcDefault":{"Latency":100000000},
	"RackDefault":{"Latency":10000000},
	"HostDefault":{"Latency":1000000},
	"DataCenters":[
		{
			"Name":"animal",
			"Racks":[
				{"Name":"land","Hosts":[{"Name":"tiger"},{"Name":"lion"},{"Name":"wolf"}]},
				{"Name":"sea","Hosts":[{"Name":"shark"},{"Name":"whale"},{"Name":"cod"}]},
				{"Name":"air","Hosts":[{"Name":"eagle"},{"Name":"crow"},{"Name":"owl"}]}
			]
		},
		{
			"Name":"plant",
			"Racks":[
				{"Name":"fruit","Hosts":[{"Name":"apple"},{"Name":"pear"},{"Name":"grape"}]},
				{"Name":"crop","Hosts":[{"Name":"corn"},{"Name":"rice"},{"Name":"wheat"}]},
				{"Name":"flower","Hosts":[{"Name":"rose"},{"Name":"lily"},{"Name":"lotus"}]}
			]
		},
		{
			"Name":"matter",
			"Racks":[
				{"Name":"metal","Hosts":[{"Name":"gold"},{"Name":"silver"},{"Name":"iron"}]},
				{"Name":"gem","Hosts":[{"Name":"ruby"},{"Name":"ivory"},{"Name":"pearl"}]},
				{"Name":"liquid","Hosts":[{"Name":"water"},{"Name":"oil"},{"Name":"wine"}]}
			]
		}
	]
}
`)
