package main

import (
	"encoding/json"
	"log"
	"sort"

	"github.com/Shopify/toxiproxy/client"
	"github.com/codegangsta/cli"

	"fmt"
	"os"
)

const (
	redColor    = "\x1b[31m"
	greenColor  = "\x1b[32m"
	yellowColor = "\x1b[33m"
	blueColor   = "\x1b[34m"
	grayColor   = "\x1b[37m"
	noColor     = "\x1b[0m"
)

func main() {
	defer fmt.Print(noColor) // make sure to clear unwanted colors
	toxiproxyClient := toxiproxy.NewClient("http://localhost:8474")

	app := cli.NewApp()
	app.Name = "Toxiproxy"
	app.Usage = "Simulate network and system conditions"
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Usage:   "list all proxies",
			Aliases: []string{"l", "li", "ls"}, // TODO would it be cleaner to limit aliases
			Action:  withToxi(list, toxiproxyClient),
		},
		{
			Name:    "inspect",
			Aliases: []string{"i", "ins"},
			Usage:   "inspect a single proxy",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name of the proxy",
				},
			},
			Action: withToxi(inspect, toxiproxyClient),
		},
		{
			Name:    "toggle",
			Usage:   "toggle enabled status on a proxy",
			Aliases: []string{"tog"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name of the proxy",
				},
			},
			Action: withToxi(toggle, toxiproxyClient),
		},
		{
			Name:    "create",
			Usage:   "create a new proxy",
			Aliases: []string{"c", "new"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name of the proxy",
				},
				cli.StringFlag{
					Name:  "listen, l",
					Usage: "proxy will listen on this address",
				},
				cli.StringFlag{
					Name:  "upstream, u",
					Usage: "proxy will forward to this address",
				},
			},
			Action: withToxi(create, toxiproxyClient),
		},
		{
			Name:    "delete",
			Usage:   "delete a proxy",
			Aliases: []string{"d"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name of the proxy",
				},
			},
			Action: withToxi(delete, toxiproxyClient),
		},
		{
			Name:    "set",
			Usage:   "set a toxic on a proxy",
			Aliases: []string{"s"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name of the proxy",
				},
				cli.StringFlag{
					Name:  "direction, d",
					Usage: "upstream or downstream", // TODO -u or -d
				},
				cli.StringFlag{
					Name:  "toxic, t",
					Usage: "kind of toxic",
				},
				cli.StringFlag{
					Name:  "fields, f",
					Usage: "key value json string",
				},
			},
			Action: withToxi(addToxic, toxiproxyClient),
		},
	}

	app.Run(os.Args)
}

type toxiAction func(*cli.Context, *toxiproxy.Client)

func withToxi(f toxiAction, t *toxiproxy.Client) func(*cli.Context) {
	return func(c *cli.Context) {
		f(c, t)
	}
}

func list(c *cli.Context, t *toxiproxy.Client) {
	proxies, err := t.Proxies()
	if err != nil {
		log.Fatalln("Failed to retrieve proxies: ", err)
	}

	var proxyNames []string
	for proxyName := range proxies {
		proxyNames = append(proxyNames, proxyName)
	}
	sort.Strings(proxyNames)

	fmt.Fprintf(os.Stderr, "%sListen\t\t%sUpstream\t%sName%s\n", blueColor, yellowColor, greenColor, noColor)
	fmt.Fprintf(os.Stderr, "%s================================================================================%s\n", grayColor, noColor)

	for _, proxyName := range proxyNames {
		proxy := proxies[proxyName]
		fmt.Printf("%s%s\t%s%s\t%s%s%s\n", blueColor, proxy.Listen, yellowColor, proxy.Upstream, enabledColor(proxy.Enabled), proxy.Name, noColor)
	}
}
func inspect(c *cli.Context, t *toxiproxy.Client) {
	proxyName := c.String("name")

	proxy, err := t.Proxy(proxyName)
	if err != nil {
		log.Fatalf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	// TODO It would be cool to include the enabled toxics in the pipe
	// rendering --L-SC-> for latency + slow close enabled
	fmt.Printf("%s%s%s\n", enabledColor(proxy.Enabled), proxy.Name, noColor)
	fmt.Printf("%s%s %s---> %s%s%s\n\n", blueColor, proxy.Listen, grayColor, yellowColor, proxy.Upstream, noColor)

	listToxics(proxy.ToxicsUpstream, "upstream")
	fmt.Println()
	listToxics(proxy.ToxicsDownstream, "downstream")
}

func toggle(c *cli.Context, t *toxiproxy.Client) {
	proxyName := c.String("name")

	proxy, err := t.Proxy(proxyName)
	if err != nil {
		log.Fatalf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	proxy.Enabled = !proxy.Enabled

	err = proxy.Save()
	if err != nil {
		log.Fatalf("Failed to toggle proxy %s: %s\n", proxyName, err.Error())
	}

	fmt.Printf("Proxy %s%s%s is now %s%s%s\n", enabledColor(proxy.Enabled), proxyName, noColor, enabledColor(proxy.Enabled), enabledText(proxy.Enabled), noColor)
}

func create(c *cli.Context, t *toxiproxy.Client) {
	proxyName := c.String("name")
	listen := c.String("listen")
	upstream := c.String("upstream")
	p := &toxiproxy.Proxy{
		Name:     proxyName,
		Listen:   listen,
		Upstream: upstream,
		Enabled:  true,
	}
	p = t.NewProxy(p)
	err := p.Create()
	if err != nil {
		log.Fatalf("Failed to create proxy: %s\n", err.Error())
	}
	fmt.Printf("Created new proxy %s\n", proxyName)
}

func delete(c *cli.Context, t *toxiproxy.Client) {
	proxyName := c.String("name")
	p, err := t.Proxy(proxyName)
	if err != nil {
		log.Fatalf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	err = p.Delete()
	if err != nil {
		log.Fatalf("Failed to delete proxy: %s\n", err.Error())
	}
	fmt.Printf("Deleted proxy %s\n", proxyName)
}

func addToxic(c *cli.Context, t *toxiproxy.Client) {
	proxyName := c.String("name")
	toxicName := c.String("toxic")
	direction := c.String("direction")
	toxicConfig := c.String("fields")
	p, err := t.Proxy(proxyName)
	if err != nil {
		log.Fatalf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}
	conf := parseToxicConfig(toxicConfig)
	_, err = p.SetToxic(toxicName, direction, conf)
	if err != nil {
		log.Fatalf("Failed to set toxic: %s\n", err.Error())
	}
	fmt.Printf("Set %s %s toxic on proxy %s\n", direction, toxicName, proxyName)
}

func parseToxicConfig(raw string) map[string]interface{} {
	parsed := map[string]interface{}{}
	err := json.Unmarshal([]byte(raw), &parsed)
	if err != nil {
		log.Fatal("Toxic config parsing error: %s\n", err)
	}
	return parsed
}

func enabledColor(enabled bool) string {
	if enabled {
		return greenColor
	}

	return redColor
}

func enabledText(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

// TODO should have upstream and downstream headings
func listToxics(toxics toxiproxy.Toxics, direction string) {
	for name, toxic := range toxics {
		fmt.Printf("%s%s direction=%s", enabledColor(toxic["enabled"].(bool)), name, direction)

		for property, value := range toxic {
			fmt.Printf(" %s=", property)
			fmt.Print(value)
		}
		fmt.Println()
	}
}
