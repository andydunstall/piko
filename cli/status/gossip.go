package status

import (
	"fmt"
	"net/url"
	"os"
	"sort"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/status/client"
	"github.com/andydunstall/pico/status/config"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newGossipCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gossip",
		Short: "inspect gossip state",
	}

	cmd.AddCommand(newGossipMembersCommand())
	cmd.AddCommand(newGossipMemberCommand())

	return cmd
}

func newGossipMembersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "inspect gossip members",
		Long: `Inspect gossip members.

Queries the server for the metadata for each known gossip member in the
cluster.

Examples:
  pico status gossip members
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8081",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showGossipMembers(&conf)
	}

	return cmd
}

type gossipMembersOutput struct {
	Members []*kite.MemberMeta `json:"members"`
}

func showGossipMembers(conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	members, err := client.GossipMembers()
	if err != nil {
		fmt.Printf("failed to get gossip members: %s\n", err.Error())
		os.Exit(1)
	}

	// Sort by ID.
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID < members[j].ID
	})

	output := gossipMembersOutput{
		Members: members,
	}
	b, _ := yaml.Marshal(output)
	fmt.Println(string(b))
}

func newGossipMemberCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Args:  cobra.ExactArgs(1),
		Short: "inspect a gossip member",
		Long: `Inspect a gossip member.

Queries the server for the known state of the gossip member with the given ID.

Examples:
  pico status gossip member bbc69214
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8081",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showGossipMember(args[0], &conf)
	}

	return cmd
}

func showGossipMember(memberID string, conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	member, err := client.GossipMember(memberID)
	if err != nil {
		fmt.Printf("failed to get gossip member: %s: %s\n", memberID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(member)
	fmt.Println(string(b))
}
