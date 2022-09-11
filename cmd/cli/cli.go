package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	terminal "golang.org/x/term"

	toxiproxyServer "github.com/Shopify/toxiproxy/v2"
	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

const (
	RED    = "\x1b[31m"
	GREEN  = "\x1b[32m"
	YELLOW = "\x1b[33m"
	BLUE   = "\x1b[34m"
	PURPLE = "\x1b[35m"
	NONE   = "\x1b[0m"
)

func color(color string) string {
	if isTTY {
		return color
	} else {
		return ""
	}
}

var toxicDescription = `
  Default Toxics:
  latency:    delay all data +/- jitter
              latency=<ms>,jitter=<ms>

  bandwidth:  limit to max kb/s
              rate=<KB/s>

  slow_close: delay from closing
              delay=<ms>

  timeout:    stop all data and close after timeout
              timeout=<ms>

  reset_peer: simulate TCP RESET (Connection reset by peer) on the connections by closing
              the stub Input immediately or after a timeout
              timeout=<ms>

  slicer:     slice data into bits with optional delay
              average_size=<bytes>,size_variation=<bytes>,delay=<microseconds>

  toxic add:
    usage: toxiproxy-cli toxic add --type <toxicType> [--downstream|--upstream] \
            --toxicName <toxicName> [--toxicity <float>] \
            --attribute <key=value> [--attribute <key2=value2>] <proxyName>


    example: toxiproxy-cli toxic add -t latency -n myToxic -a latency=100 -a jitter=50 myProxy

  toxic update:
    usage: toxiproxy-cli toxic update --toxicName <toxicName> [--toxicity <float>] \
            --attribute <key1=value1> [--attribute <key2=value2>] <proxyName>

    example: toxiproxy-cli toxic update -n myToxic -a jitter=25 myProxy

  toxic delete:
    usage: toxiproxy-cli toxic delete --toxicName <toxicName> <proxyName>

    example: toxiproxy-cli toxic delete -n myToxic myProxy
`

var (
	hostname string
	isTTY    bool
)

func main() {
	app := cli.NewApp()
	app.Name = "toxiproxy-cli"
	app.Version = toxiproxyServer.Version
	app.Usage = "Simulate network and system conditions"
	app.Commands = cliCommands()
	cli.HelpFlag = &cli.BoolFlag{
		Name:  "help",
		Usage: "show help",
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "host",
			Aliases:     []string{"h"},
			Value:       "http://localhost:8474",
			Usage:       "toxiproxy host to connect to",
			Destination: &hostname,
			EnvVars:     []string{"TOXIPROXY_URL"},
		},
	}

	isTTY = terminal.IsTerminal(int(os.Stdout.Fd()))

	app.Run(os.Args)
}

func cliCommands() []*cli.Command {
	return []*cli.Command{
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
			Name: "create",
			Usage: "create a new proxy\n\t" +
				"usage: 'toxiproxy-cli create --listen <addr> --upstream <addr> <proxyName>'\n",
			Aliases: []string{"c", "new"},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "listen",
					Aliases: []string{"l"},
					Usage:   "proxy will listen on this address",
				},
				&cli.StringFlag{
					Name:    "upstream",
					Aliases: []string{"u"},
					Usage:   "proxy will forward to this address",
				},
			},
			Action: withToxi(createProxy),
		},
		{
			Name: "toggle",
			Usage: "\ttoggle enabled status on a proxy\n" +
				"\t\tusage: 'toxiproxy-cli toggle <proxyName>'\n",
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
			Subcommands: cliToxiSubCommands(),
		},
	}
}

func cliToxiSubCommands() []*cli.Command {
	return []*cli.Command{
		cliToxiAddSubCommand(),
		cliToxiUpdateSubCommand(),
		cliToxiRemoveSubCommand(),
	}
}

func cliToxiAddSubCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Aliases:   []string{"a"},
		Usage:     "add a new toxic",
		ArgsUsage: "<proxyName>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "toxicName",
				Aliases: []string{"n"},
				Usage:   "name of the toxic",
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "type of toxic",
			},
			&cli.StringFlag{
				Name:        "toxicity",
				Aliases:     []string{"tox"},
				Usage:       "toxicity of toxic should be a float between 0 and 1",
				DefaultText: "1.0",
			},
			&cli.StringSliceFlag{
				Name:    "attribute",
				Aliases: []string{"a"},
				Usage:   "toxic attribute in key=value format",
			},
			&cli.BoolFlag{
				Name:        "upstream",
				Aliases:     []string{"u"},
				Usage:       "add toxic to upstream",
				DefaultText: "false",
			},
			&cli.BoolFlag{
				Name:        "downstream",
				Aliases:     []string{"d"},
				Usage:       "add toxic to downstream",
				DefaultText: "true",
			},
		},
		Action: withToxi(addToxic),
	}
}

func cliToxiUpdateSubCommand() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Aliases:   []string{"u"},
		Usage:     "update an enabled toxic",
		ArgsUsage: "<proxyName>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "toxicName",
				Aliases: []string{"n"},
				Usage:   "name of the toxic",
			},
			&cli.StringFlag{
				Name:        "toxicity",
				Aliases:     []string{"tox"},
				Usage:       "toxicity of toxic should be a float between 0 and 1",
				DefaultText: "1.0",
			},
			&cli.StringSliceFlag{
				Name:    "attribute",
				Aliases: []string{"a"},
				Usage:   "toxic attribute in key=value format",
			},
		},
		Action: withToxi(updateToxic),
	}
}

func cliToxiRemoveSubCommand() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Aliases:   []string{"r", "delete", "d"},
		Usage:     "remove an enabled toxic",
		ArgsUsage: "<proxyName>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "toxicName",
				Aliases: []string{"n"},
				Usage:   "name of the toxic",
			},
		},
		Action: withToxi(removeToxic),
	}
}

type toxiAction func(*cli.Context, *toxiproxy.Client) error

func withToxi(f toxiAction) func(*cli.Context) error {
	return func(c *cli.Context) error {
		toxiproxyClient := toxiproxy.NewClient(hostname)
		toxiproxyClient.UserAgent = fmt.Sprintf(
			"toxiproxy-cli/%s (%s/%s)",
			c.App.Version,
			runtime.GOOS,
			runtime.GOARCH,
		)
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

	if isTTY {
		fmt.Printf(
			"%sName\t\t\t%sListen\t\t%sUpstream\t\t%sEnabled\t\t%sToxics\n%s",
			color(GREEN),
			color(BLUE),
			color(YELLOW),
			color(PURPLE),
			color(RED),
			color(NONE),
		)
		fmt.Printf(
			"%s======================================================================================\n",
			color(NONE),
		)

		if len(proxyNames) == 0 {
			fmt.Printf("%sno proxies\n%s", color(RED), color(NONE))
			hint("create a proxy with `toxiproxy-cli create`")
			return nil
		}
	}

	for _, proxyName := range proxyNames {
		proxy := proxies[proxyName]
		numToxics := strconv.Itoa(len(proxy.ActiveToxics))
		if numToxics == "0" && isTTY {
			numToxics = "None"
		}
		printWidth(color(colorEnabled(proxy.Enabled)), proxy.Name, 3)
		printWidth(BLUE, proxy.Listen, 2)
		printWidth(YELLOW, proxy.Upstream, 3)
		printWidth(PURPLE, enabledText(proxy.Enabled), 2)
		fmt.Printf("%s%s%s\n", color(RED), numToxics, color(NONE))
	}
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

	if isTTY {
		fmt.Printf("%sName: %s%s\t", color(PURPLE), color(NONE), proxy.Name)
		fmt.Printf("%sListen: %s%s\t", color(BLUE), color(NONE), proxy.Listen)
		fmt.Printf("%sUpstream: %s%s\n", color(YELLOW), color(NONE), proxy.Upstream)
		fmt.Printf(
			"%s======================================================================\n",
			color(NONE),
		)

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
			fmt.Printf("%sProxy has no toxics enabled.\n%s", color(RED), color(NONE))
		} else {
			up, down := splitToxics(proxy.ActiveToxics)
			listToxics(up, "Upstream")
			fmt.Println()
			listToxics(down, "Downstream")
		}

		hint("add a toxic with `toxiproxy-cli toxic add`")
	} else {
		listToxics(proxy.ActiveToxics, "")
	}
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

	fmt.Printf(
		"Proxy %s%s%s is now %s%s%s\n",
		colorEnabled(proxy.Enabled),
		proxyName,
		color(NONE),
		colorEnabled(proxy.Enabled),
		enabledText(proxy.Enabled),
		color(NONE),
	)
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
	toxicity := defaultToxicity
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
	toxicParams, err := parseAddToxicParams(c)
	if err != nil {
		return err
	}

	toxic, err := t.AddToxic(toxicParams)
	if err != nil {
		return errorf("Failed to add toxic: %v\n", err)
	}

	fmt.Printf(
		"Added %s %s toxic '%s' on proxy '%s'\n",
		toxic.Stream,
		toxic.Type,
		toxic.Name,
		toxicParams.ProxyName,
	)

	return nil
}

func updateToxic(c *cli.Context, t *toxiproxy.Client) error {
	toxicParams, err := parseUpdateToxicParams(c)
	if err != nil {
		return err
	}

	toxic, err := t.UpdateToxic(toxicParams)
	if err != nil {
		return errorf("Failed to update toxic: %v\n", err)
	}

	fmt.Printf(
		"Updated toxic '%s' on proxy '%s'\n",
		toxic.Name,
		toxicParams.ProxyName,
	)
	return nil
}

func removeToxic(c *cli.Context, t *toxiproxy.Client) error {
	toxicParams, err := parseToxicCommonParams(c)
	if err != nil {
		return err
	}

	err = t.RemoveToxic(toxicParams)
	if err != nil {
		return errorf("Failed to remove toxic: %v\n", err)
	}

	fmt.Printf("Removed toxic '%s' on proxy '%s'\n", toxicParams.ToxicName, toxicParams.ProxyName)
	return nil
}

func parseToxicCommonParams(context *cli.Context) (*toxiproxy.ToxicOptions, error) {
	proxyName := context.Args().First()
	if proxyName == "" {
		cli.ShowSubcommandHelp(context)
		return nil, errorf("Proxy name is missing.\n")
	}

	toxicName := context.String("toxicName")

	return &toxiproxy.ToxicOptions{
		ProxyName: proxyName,
		ToxicName: toxicName,
	}, nil
}

func parseUpdateToxicParams(c *cli.Context) (*toxiproxy.ToxicOptions, error) {
	result, err := parseToxicCommonParams(c)
	if err != nil {
		return nil, err
	}

	result.Toxicity, err = parseToxicity(c, 1.0)
	if err != nil {
		return nil, err
	}

	result.Attributes = parseAttributes(c, "attribute")

	return result, nil
}

func parseAddToxicParams(c *cli.Context) (*toxiproxy.ToxicOptions, error) {
	result, err := parseToxicCommonParams(c)
	if err != nil {
		return nil, err
	}

	result.ToxicType, err = getArgOrFail(c, "type")
	if err != nil {
		return nil, err
	}

	upstream := c.Bool("upstream")
	downstream := c.Bool("downstream")
	if upstream && downstream {
		return nil, errorf("Only one should be specified: upstream or downstream.\n")
	}

	stream := "downstream"
	if upstream {
		stream = "upstream"
	}
	result.Stream = stream

	result.Toxicity, err = parseToxicity(c, 1.0)
	if err != nil {
		return nil, err
	}

	result.Attributes = parseAttributes(c, "attribute")

	return result, nil
}

func parseAttributes(c *cli.Context, name string) toxiproxy.Attributes {
	parsed := map[string]interface{}{}
	args := c.StringSlice(name)

	for _, raw := range args {
		kv := strings.SplitN(raw, "=", 2)
		if len(kv) < 2 {
			continue
		}
		if float, err := strconv.ParseFloat(kv[1], 64); err == nil {
			parsed[kv[0]] = float
		} else {
			parsed[kv[0]] = kv[1]
		}
	}
	return parsed
}

func colorEnabled(enabled bool) string {
	if enabled {
		return color(GREEN)
	}

	return color(RED)
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
	if isTTY {
		fmt.Printf("%s%s toxics:\n%s", color(GREEN), stream, color(NONE))
		if len(toxics) == 0 {
			fmt.Printf("%sProxy has no %s toxics enabled.\n%s", color(RED), stream, color(NONE))
			return
		}
	}
	for _, t := range toxics {
		if isTTY {
			fmt.Printf("%s%s:%s\t", color(BLUE), t.Name, color(NONE))
		} else {
			fmt.Printf("%s\t", t.Name)
		}
		fmt.Printf("type=%s\t", t.Type)
		fmt.Printf("stream=%s\t", t.Stream)
		fmt.Printf("toxicity=%.2f\t", t.Toxicity)
		fmt.Printf("attributes=[")
		sorted := sortedAttributes(t.Attributes)
		for _, a := range sorted {
			fmt.Printf("\t%s=", a.key)
			fmt.Print(a.value)
		}
		fmt.Printf("\t]\n")
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
	if isTTY {
		fmt.Printf("\n%sHint: %s\n", color(NONE), m)
	}
}

func errorf(m string, args ...interface{}) error {
	return cli.Exit(fmt.Sprintf(m, args...), 1)
}

func printWidth(col string, m string, numTabs int) {
	if isTTY {
		numTabs -= len(m)/8 + 1
		if numTabs < 0 {
			numTabs = 0
		}
	} else {
		numTabs = 0
	}
	fmt.Printf("%s%s%s\t%s", color(col), m, color(NONE), strings.Repeat("\t", numTabs))
}
