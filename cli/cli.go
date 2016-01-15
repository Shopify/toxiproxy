package main

import (
	"github.com/Shopify/toxiproxy/client"
	"github.com/codegangsta/cli"
	"sort"

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
			Aliases: []string{"l", "li", "ls"},
			Usage:   "List all proxies",
			Action: func(c *cli.Context) {
				proxies, err := toxiproxyClient.Proxies()
				if err != nil {
					fmt.Println("Failed to retrieve proxies: ", err)
					os.Exit(1)
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
			},
		},
		{
			Name:    "inspect",
			Aliases: []string{"i", "ins"},
			Usage:   "Inspect a single proxy",
			Action: func(c *cli.Context) {
				proxyName := c.Args().First()

				proxy, err := toxiproxyClient.Proxy(proxyName)
				if err != nil {
					fmt.Printf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
					os.Exit(1)
				}

				// TODO It would be cool to include the enabled toxics in the pipe
				// rendering --L-SC-> for latency + slow close enabled
				fmt.Printf("%s%s%s\n", enabledColor(proxy.Enabled), proxy.Name, noColor)
				fmt.Printf("%s%s %s---> %s%s%s\n\n", blueColor, proxy.Listen, grayColor, yellowColor, proxy.Upstream, noColor)

				listToxics(proxy.ToxicsUpstream, "upstream")
				fmt.Println()
				listToxics(proxy.ToxicsDownstream, "downstream")
			},
		},
		{
			Name:    "toggle",
			Usage:   "Toggle enabled status on a proxy",
			Aliases: []string{"tog"},
			Action: func(c *cli.Context) {
				proxyName := c.Args().First()

				proxy, err := toxiproxyClient.Proxy(proxyName)
				if err != nil {
					fmt.Printf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
					os.Exit(1)
				}

				proxy.Enabled = !proxy.Enabled

				err = proxy.Save()
				if err != nil {
					fmt.Printf("Failed to toggle proxy %s: %s\n", proxyName, err.Error())
					os.Exit(1)
				}

				fmt.Printf("Proxy %s%s%s is now %s%s%s\n", enabledColor(proxy.Enabled), proxyName, noColor, enabledColor(proxy.Enabled), enabledText(proxy.Enabled), noColor)
			},
		},
	}

	app.Run(os.Args)
}

func enabledColor(enabled bool) string {
	if enabled {
		return "\x1b[32m"
	}

	return "\x1b[31m"
}

func enabledText(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

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
