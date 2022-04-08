package server

// DONTCOVER

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/cli"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/p2p"
	pvm "github.com/tendermint/tendermint/privval"
	tversion "github.com/tendermint/tendermint/version"
	yaml "gopkg.in/yaml.v2"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ShowNodeIDCmd - ported from Tendermint, dump node ID to stdout
func ShowNodeIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show-node-id",
		Short: "Show this node's ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCtx := GetServerContextFromCmd(cmd)
			cfg := serverCtx.Config

			nodeKey, err := p2p.LoadNodeKey(cfg.NodeKeyFile())
			if err != nil {
				return err
			}

			cmd.Println(nodeKey.ID())
			return nil
		},
	}
}

// ShowValidatorCmd - ported from Tendermint, show this node's validator info
func ShowValidatorCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "show-validator",
		Short: "Show this node's tendermint validator info",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCtx := GetServerContextFromCmd(cmd)
			cfg := serverCtx.Config

			privValidator := pvm.LoadFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile())
			valPubKey, err := privValidator.GetPubKey()
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString(cli.OutputFlag)
			if strings.ToLower(output) == "json" {
				marshal, err := tmjson.Marshal(valPubKey.(ed25519.PubKey))
				if err != nil {
					return err
				}

				cmd.Println(string(marshal))
				return nil
			}

			pubkey, err := cryptocodec.FromTmPubKeyInterface(valPubKey)
			if err != nil {
				return err
			}
			pubkeyBech32, err := sdk.Bech32ifyPubKey(sdk.Bech32PubKeyTypeConsPub, pubkey)
			if err != nil {
				return err
			}

			cmd.Println(pubkeyBech32)
			return nil
		},
	}

	cmd.Flags().StringP(cli.OutputFlag, "o", "text", "Output format (text|json)")
	return &cmd
}

// ShowAddressCmd - show this node's validator address
func ShowAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-address",
		Short: "Shows this node's tendermint validator consensus address",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCtx := GetServerContextFromCmd(cmd)
			cfg := serverCtx.Config

			privValidator := pvm.LoadFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile())
			valConsAddr := (sdk.ConsAddress)(privValidator.GetAddress())

			cmd.Println(valConsAddr.String())
			return nil
		},
	}
	return cmd
}

// VersionCmd prints tendermint and ABCI version numbers.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print tendermint libraries' version",
		Long: `Print protocols' and libraries' version numbers
against which this app has been compiled.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bs, err := yaml.Marshal(&struct {
				Tendermint    string
				ABCI          string
				BlockProtocol uint64
				P2PProtocol   uint64
			}{
				Tendermint:    tversion.TMCoreSemVer,
				ABCI:          tversion.ABCIVersion,
				BlockProtocol: tversion.BlockProtocol,
				P2PProtocol:   tversion.P2PProtocol,
			})
			if err != nil {
				return err
			}

			cmd.Println(string(bs))
			return nil
		},
	}
}
