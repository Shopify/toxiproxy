package main

import (
	"sort"
	"strconv"
	"strings"

	toxiproxyServer "github.com/Shopify/toxiproxy"
	"github.com/Shopify/toxiproxy/client"
	"github.com/urfave/cli"

	"fmt"
	"os"
)

const (
	redColor    = "\x1b[31m"
	greenColor  = "\x1b[32m"
	yellowColor = "\x1b[33m"
	blueColor   = "\x1b[34m"
	cyanColor   = "\x1b[36m"
	purpleColor = "\x1b[35m"
	grayColor   = "\x1b[37m"
	noColor     = "\x1b[0m"
)

var toxicDescription = `
	Default Toxics:
	latency:	delay all data +/- jitter
		latency=<ms>,jitter=<ms>

	bandwidth:	limit to max kb/s
		rate=<KB/s>

	slow_close:	delay from closing
		delay=<ms>

	timeout: 	stop all data and close after timeout
	         	timeout=<ms>

	slicer: 	slice data into bits with optional delay
	        	average_size=<byes>,size_variation=<bytes>,delay=<microseconds>

	toxic add:
		usage: toxiproxy-cli add <proxyName> --type <toxicType> --toxicName <toxicName> \
		--attributes <key1=value1,key2=value2...> --upstream --downstream

		example: toxiproxy-cli toxic add myProxy -t latency -n myToxic -f latency=100,jitter=50

	toxic update:
		usage: toxiproxy-cli update <proxyName> --toxicName <toxicName> \
		--attributes <key1=value1,key2=value2...>

		example: toxiproxy-cli toxic update myProxy -n myToxic -f jitter=25

	toxic delete:
		usage: toxiproxy-cli update <proxyName> --toxicName <toxicName>

		example: toxiproxy-cli toxic delete myProxy -n myToxic
`

var hostname string

func main() {
	app := cli.NewApp()
	app.Name = "toxiproxy-cli"
	app.Version = toxiproxyServer.Version
	app.Usage = "Simulate network and system conditions"
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Usage:   "list all proxies\n\tusage: 'toxiproxy-cli list'\n",
			Aliases: []string{"l", "li", "ls"},
			Action:  withToxi(list),
		},
		{
			Name:    "inspect",
			Aliases: []string{"i", "ins"},
			Usage:   "inspect a single proxy\n\tusage: 'toxiproxy-cli inspect <proxyName>'\n",
			Action:  withToxi(inspectProxy),
		},
		{
			Name:    "create",
			Usage:   "create a new proxy\n\tusage: 'toxiproxy-cli create <proxyName> --listen <addr> --upstream <addr>'\n",
			Aliases: []string{"c", "new"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "listen, l",
					Usage: "proxy will listen on this address",
				},
				cli.StringFlag{
					Name:  "upstream, u",
					Usage: "proxy will forward to this address",
				},
			},
			Action: withToxi(createProxy),
		},
		{
			Name:    "toggle",
			Usage:   "\ttoggle enabled status on a proxy\n\t\tusage: 'toxiproxy-cli toggle <proxyName>'\n",
			Aliases: []string{"tog"},
			Action:  withToxi(toggleProxy),
		},
		{
			Name:    "delete",
			Usage:   "\tdelete a proxy\n\t\tusage: 'toxiproxy-cli delete <proxyName>'\n",
			Aliases: []string{"d"},
			Action:  withToxi(deleteProxy),
		},
		{
			Name:        "toxic",
			Aliases:     []string{"t"},
			Usage:       "\tadd, remove or update a toxic\n\t\tusage: see 'toxiproxy-cli toxic'\n",
			Description: toxicDescription,
			Subcommands: []cli.Command{
				{
					Name:      "add",
					Aliases:   []string{"a"},
					Usage:     "add a new toxic",
					ArgsUsage: "<proxyName>",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "toxicName, n",
							Usage: "name of the toxic",
						},
						cli.StringFlag{
							Name:  "type, t",
							Usage: "type of toxic",
						},
						cli.StringFlag{
							Name:  "toxicity, tox",
							Usage: "toxicity of toxic",
						},
						cli.StringFlag{
							Name:  "attributes, a",
							Usage: "comma seperated key=value toxic attributes",
						},
						cli.BoolFlag{
							Name:  "upstream, u",
							Usage: "add toxic to upstream",
						},
						cli.BoolFlag{
							Name:  "downstream, d",
							Usage: "add toxic to downstream",
						},
					},
					Action: withToxi(addToxic),
				},
				{
					Name:      "update",
					Aliases:   []string{"u"},
					Usage:     "update an enabled toxic",
					ArgsUsage: "<proxyName>",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "toxicName, n",
							Usage: "name of the toxic",
						},
						cli.StringFlag{
							Name:  "toxicity, tox",
							Usage: "toxicity of toxic",
						},
						cli.StringFlag{
							Name:  "attributes, a",
							Usage: "comma seperated key=value toxic attributes",
						},
					},
					Action: withToxi(updateToxic),
				},
				{
					Name:      "remove",
					Aliases:   []string{"r", "delete", "d"},
					Usage:     "remove an enabled toxic",
					ArgsUsage: "<proxyName>",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "toxicName, n",
							Usage: "name of the toxic",
						},
					},
					Action: withToxi(removeToxic),
				},
			},
		},
	}
	cli.HelpFlag = cli.BoolFlag{
		Name:  "help",
		Usage: "show help",
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "host, h",
			Value:       "http://localhost:8474",
			Usage:       "toxiproxy host to connect to",
			Destination: &hostname,
		},
	}

	app.Run(os.Args)
}

type toxiAction func(*cli.Context, *toxiproxy.Client) error

func withToxi(f toxiAction) func(*cli.Context) error {
	return func(c *cli.Context) error {
		toxiproxyClient := toxiproxy.NewClient(hostname)
		return f(c, toxiproxyClient)
	}
}

func list(c *cli.Context, t *toxiproxy.Client) error {
	proxies, err := t.Proxies()
	if err != nil {
		return errorf("Failed to retrieve proxies: %s", err)
	}

	var proxyNames []string
	for proxyName := range proxies {
		proxyNames = append(proxyNames, proxyName)
	}
	sort.Strings(proxyNames)

	fmt.Fprintf(os.Stderr, "%sListen\t\t%sUpstream\t%sName\t%sEnabled\t%sToxics\n%s", blueColor, yellowColor,
		greenColor, purpleColor, redColor, noColor)
	fmt.Fprintf(os.Stderr, "%s======================================================================\n", noColor)

	if len(proxyNames) == 0 {
		fmt.Printf("%sno proxies\n\n%s", redColor, noColor)
		hint("create a proxy with `toxiproxy-cli create`")
		return nil
	}

	for _, proxyName := range proxyNames {
		proxy := proxies[proxyName]
		numToxics := strconv.Itoa(len(proxy.ActiveToxics))
		if numToxics == "0" {
			numToxics = "None"
		}
		fmt.Printf("%s%s\t%s%s\t%s%s\t%s%v\t%s%s%s\n", blueColor, proxy.Listen, yellowColor, proxy.Upstream,
			enabledColor(proxy.Enabled), proxy.Name, purpleColor, proxy.Enabled, redColor, numToxics, noColor)
	}
	fmt.Println()
	hint("inspect toxics with `toxiproxy-cli inspect <proxyName>`")
	return nil
}

func inspectProxy(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}

	proxy, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	fmt.Printf("%sName: %s%s\t", purpleColor, noColor, proxy.Name)
	fmt.Printf("%sListen: %s%s\t", blueColor, noColor, proxy.Listen)
	fmt.Printf("%sUpstream: %s%s\n", yellowColor, noColor, proxy.Upstream)
	fmt.Printf("%s======================================================================\n", noColor)

	splitToxics := func(toxics toxiproxy.Toxics) (toxiproxy.Toxics, toxiproxy.Toxics) {
		upstream := make(toxiproxy.Toxics, 0)
		downstream := make(toxiproxy.Toxics, 0)
		for _, toxic := range toxics {
			if toxic.Stream == "upstream" {
				upstream = append(upstream, toxic)
			} else {
				downstream = append(downstream, toxic)
			}
		}
		return upstream, downstream
	}

	if len(proxy.ActiveToxics) == 0 {
		fmt.Printf("%sProxy has no toxics enabled.\n%s", redColor, noColor)
	} else {
		up, down := splitToxics(proxy.ActiveToxics)
		listToxics(up, "Upstream")
		fmt.Println()
		listToxics(down, "Downstream")
	}
	fmt.Println()

	hint("add a toxic with `toxiproxy-cli toxic add`")
	return nil
}

func toggleProxy(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}

	proxy, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	proxy.Enabled = !proxy.Enabled

	err = proxy.Save()
	if err != nil {
		return errorf("Failed to toggle proxy %s: %s\n", proxyName, err.Error())
	}

	fmt.Printf("Proxy %s%s%s is now %s%s%s\n", enabledColor(proxy.Enabled), proxyName, noColor, enabledColor(proxy.Enabled), enabledText(proxy.Enabled), noColor)
	return nil
}

func createProxy(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}
	listen, err := getArgOrFail(c, "listen")
	if err != nil {
		return err
	}
	upstream, err := getArgOrFail(c, "upstream")
	if err != nil {
		return err
	}
	_, err = t.CreateProxy(proxyName, listen, upstream)
	if err != nil {
		return errorf("Failed to create proxy: %s\n", err.Error())
	}
	fmt.Printf("Created new proxy %s\n", proxyName)
	return nil
}

func deleteProxy(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}
	p, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	err = p.Delete()
	if err != nil {
		return errorf("Failed to delete proxy: %s\n", err.Error())
	}
	fmt.Printf("Deleted proxy %s\n", proxyName)
	return nil
}

func parseToxicity(c *cli.Context, defaultToxicity float32) (float32, error) {
	var toxicity = defaultToxicity
	toxicityString := c.String("toxicity")
	if toxicityString != "" {
		tox, err := strconv.ParseFloat(toxicityString, 32)
		if err != nil || tox > 1 || tox < 0 {
			return 0, errorf("toxicity should be a float between 0 and 1.\n")
		}
		toxicity = float32(tox)
	}
	return toxicity, nil
}

func addToxic(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}
	toxicName := c.String("toxicName")
	toxicType, err := getArgOrFail(c, "type")
	if err != nil {
		return err
	}
	toxicAttributes, err := getArgOrFail(c, "attributes")
	if err != nil {
		return err
	}

	upstream := c.Bool("upstream")
	downstream := c.Bool("downstream")

	toxicity, err := parseToxicity(c, 1.0)
	if err != nil {
		return err
	}

	attributes, err := parseAttributes(toxicAttributes)
	if err != nil {
		return err
	}

	p, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	addToxic := func(stream string) error {
		t, err := p.AddToxic(toxicName, toxicType, stream, toxicity, attributes)
		if err != nil {
			return errorf("Failed to add toxic: %s\n", err.Error())
		}
		toxicName = t.Name
		fmt.Printf("Added %s %s toxic '%s' on proxy '%s'\n", stream, toxicType, toxicName, proxyName)
		return nil
	}

	if upstream {
		err := addToxic("upstream")
		if err != nil {
			return err
		}
	}
	// Default to downstream.
	if downstream || (!downstream && !upstream) {
		return addToxic("downstream")
	}
	return nil
}

func updateToxic(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}
	toxicName, err := getArgOrFail(c, "toxicName")
	if err != nil {
		return err
	}
	toxicAttributes, err := getArgOrFail(c, "attributes")
	if err != nil {
		return err
	}

	attributes, err := parseAttributes(toxicAttributes)
	if err != nil {
		return err
	}

	p, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	toxicity, err := parseToxicity(c, -1)
	if err != nil {
		return nil
	}

	_, err = p.UpdateToxic(toxicName, toxicity, attributes)
	if err != nil {
		return errorf("Failed to update toxic: %s\n", err.Error())
	}

	fmt.Printf("Updated toxic '%s' on proxy '%s'\n", toxicName, proxyName)
	return nil
}

func removeToxic(c *cli.Context, t *toxiproxy.Client) error {
	proxyName := c.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(c)
		return errorf("Proxy name is required as the first argument.\n")
	}
	toxicName, err := getArgOrFail(c, "toxicName")
	if err != nil {
		return err
	}

	p, err := t.Proxy(proxyName)
	if err != nil {
		return errorf("Failed to retrieve proxy %s: %s\n", proxyName, err.Error())
	}

	err = p.RemoveToxic(toxicName)
	if err != nil {
		return errorf("Failed to remove toxic: %s\n", err.Error())
	}

	fmt.Printf("Removed toxic '%s' on proxy '%s'\n", toxicName, proxyName)
	return nil
}

func parseAttributes(raw string) (toxiproxy.Attributes, error) {
	parsed := map[string]interface{}{}
	keyValues := strings.Split(raw, ",")
	for _, keyValue := range keyValues {
		kv := strings.SplitN(keyValue, "=", 2)
		if len(kv) != 2 {
			return nil, errorf("Attributes should be in format key1=value1,key2=value2\n")
		}
		value, err := strconv.Atoi(kv[1])
		if err != nil {
			return nil, errorf("Toxic attributes was expected to be an integer.\n")
		}
		parsed[kv[0]] = value
	}
	return parsed, nil
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

type attribute struct {
	key   string
	value interface{}
}

type attributeList []attribute

func (a attributeList) Len() int           { return len(a) }
func (a attributeList) Less(i, j int) bool { return a[i].key < a[j].key }
func (a attributeList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func sortedAttributes(attrs toxiproxy.Attributes) attributeList {
	li := make(attributeList, 0, len(attrs))
	for k, v := range attrs {
		li = append(li, attribute{k, v.(float64)})
	}
	sort.Sort(li)
	return li
}

func listToxics(toxics toxiproxy.Toxics, stream string) {
	fmt.Printf("%s%s toxics:\n%s", greenColor, stream, noColor)
	if len(toxics) == 0 {
		fmt.Printf("%sProxy has no %s toxics enabled.\n%s", redColor, stream, noColor)
		return
	}
	for _, t := range toxics {
		fmt.Printf("%s%s:%s ", redColor, t.Name, noColor)
		fmt.Printf("type=%s ", t.Type)
		fmt.Printf("stream=%s ", t.Stream)
		fmt.Printf("toxicity=%.2f ", t.Toxicity)
		fmt.Printf("attributes=[")
		sorted := sortedAttributes(t.Attributes)
		for _, a := range sorted {
			fmt.Printf(" %s=", a.key)
			fmt.Print(a.value)
		}
		fmt.Printf(" ]\n")
	}
}

func getArgOrFail(c *cli.Context, name string) (string, error) {
	arg := c.String(name)
	if arg == "" {
		cli.ShowSubcommandHelp(c)
		return "", errorf("Required argument '%s' was empty.\n", name)
	}
	return arg, nil
}

func hint(m string) {
	fmt.Printf("%sHint: %s\n", noColor, m)
}

func errorf(m string, args ...interface{}) error {
	return cli.NewExitError(fmt.Sprintf(m, args...), 1)
}
