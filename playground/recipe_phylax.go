package playground

import (
	"fmt"
	"strings"

	"log"

	flag "github.com/spf13/pflag"
)

var _ Recipe = &OpTalosRecipe{}

// OpRecipe is a recipe that deploys an OP stack
type OpTalosRecipe struct {
	// externalBuilder is the URL of the external builder to use. If set, the recipe deploys
	// rollup-boost on the sequencer and uses this URL as the external builder.
	externalBuilder string

	// whether to enable the latest fork isthmus and when
	enableLatestFork *uint64

	// blockTime is the block time to use for the rollup
	// (default is 2 seconds)
	blockTime uint64

	// batcherMaxChannelDuration is the maximum channel duration to use for the batcher
	// (default is 2 seconds)
	batcherMaxChannelDuration uint64

	// externalDA is the URL of the external DA to use. If unset or set to "dev", the recipe deploys
	// a new assertion-da service and connects it to the external DA.
	externalDA string

	// assertionDAImageTag is the docker image tag for assertion-da
	assertionDAImageTag string

	// assertionDAPrivateKey is the private key of the assertion DA
	assertionDAPrivateKey string

	// assexGasLimit is the gas limit of the Assertion Execution
	assexGasLimit uint64

	// oracleContract is the address of the State Oracle contract
	oracleContract string

	// Enable the faucet UI
	faucetUi bool

	// faucetPrivateKey is the private key of the faucet address
	faucetPrivateKey string

	// opTalosImageTag is the docker image tag for op-talos
	opTalosImageTag string

	// opTalosBlockTag is the block tag to be used by OP-Talos for Assertion Indexing
	opTalosBlockTag string
}

func (o *OpTalosRecipe) Name() string {
	return "pcl"
}

func (o *OpTalosRecipe) Description() string {
	return "Deploy OP Talos"
}

func (o *OpTalosRecipe) Flags() *flag.FlagSet {
	flags := flag.NewFlagSet("pcl", flag.ContinueOnError)
	flags.StringVar(&o.externalBuilder, "external-builder", "", "External builder URL")
	flags.StringVar(&o.externalDA, "external-da", "", "External DA URL")
	flags.StringVar(&o.assertionDAImageTag, "assertion-da-image-tag", "", "assertion-da docker image specification in 'imagename:tag' format. If provided, both imagename and tag must be non-empty.")
	// Default: $(cast keccak "credible-layer-sandbox-assertion-da") -> Address: 0xEc64B5CC78f8f0f2d17Fa2D48DdEFc9abdf68c2B
	flags.StringVar(&o.assertionDAPrivateKey, "assertion-da-private-key", "0xb14a93020522e378f565ebd6d1032b06af46dc778a1bfea9602a9641547c4673", "Private key for assertion DA (required if external-da is unset, empty, or 'dev')")
	flags.Var(&nullableUint64Value{&o.enableLatestFork}, "enable-latest-fork", "Enable latest fork isthmus (nil or empty = disabled, otherwise enabled at specified block)")
	flags.Uint64Var(&o.blockTime, "block-time", defaultOpBlockTimeSeconds, "Block time to use for the rollup")
	flags.Uint64Var(&o.batcherMaxChannelDuration, "batcher-max-channel-duration", 2, "Maximum channel duration to use for the batcher")
	flags.Uint64Var(&o.assexGasLimit, "assex-gas-limit", 30000000, "Gas limit of the Assertion Execution")
	flags.StringVar(&o.oracleContract, "oracle-contract", "0x6dD3f12ce435f69DCeDA7e31605C02Bb5422597b", "State Oracle contract address")
	flags.BoolVar(&o.faucetUi, "faucet-ui", false, "Enable the faucet UI")
	// Default: $(cast keccak "credible-layer-sandbox-faucet") -> Address: 0xA242C9e875a3135644a171CE7e0d44A14511F897
	flags.StringVar(&o.faucetPrivateKey, "faucet-private-key", "0x0263f53e0add655d0caa4daaeaf8aa749689beed953a902fc16adf3b944e7fd4", "Private key for faucet")
	flags.StringVar(&o.opTalosImageTag, "op-talos-image-tag", "", "op-talos docker image specification in 'imagename:tag' format. If provided, both imagename and tag must be non-empty.")
	flags.StringVar(&o.opTalosBlockTag, "op-talos-block-tag", "latest", "The block tag (finalized, safe, latest) to be used by OP-Talos for Assertion Indexing")
	return flags
}

func (o *OpTalosRecipe) Artifacts() *ArtifactsBuilder {
	builder := NewArtifactsBuilder()
	builder.ApplyLatestL2Fork(o.enableLatestFork)
	builder.OpBlockTime(o.blockTime)
	builder.PrefundedAccounts([]string{
		o.faucetPrivateKey,
	})

	return builder
}

func (o *OpTalosRecipe) Apply(ctx *ExContext, artifacts *Artifacts) (*Manifest, error) {

	parsedImageNameAssertionDa := "ghcr.io/phylaxsystems/assertion-da/assertion-da" // Default image name
	parsedImageTagAssertionDa := "latest"                                           // Default image tag

	// Validate required flags
	if o.externalDA == "" {
		if o.assertionDAPrivateKey == "" {
			panic("assertion-da-private-key is required when external-da is unset or empty")
		}
	}

	if o.assertionDAImageTag != "" { // If the flag was provided
		parts := strings.SplitN(o.assertionDAImageTag, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --assertion-da-image-tag value: '%s'. Must be in 'imagename:tag' format with non-empty imagename and tag parts", o.assertionDAImageTag)
		}
		parsedImageNameAssertionDa = parts[0]
		parsedImageTagAssertionDa = parts[1]
	}

	parsedImageNameOpTalos := "ghcr.io/phylaxsystems/op-talos/op-rbuilder" // Default image name
	parsedImageTagOpTalos := "master"                                      // Default image tag

	if o.opTalosImageTag != "" { // If the flag was provided
		parts := strings.SplitN(o.opTalosImageTag, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --op-talos-image-tag value: '%s'. Must be in 'imagename:tag' format with non-empty imagename and tag parts", o.opTalosImageTag)
		}
		parsedImageNameOpTalos = parts[0]
		parsedImageTagOpTalos = parts[1]
	}

	svcManager := NewManifest(ctx, artifacts.Out)
	svcManager.AddService("el", &RethEL{})
	svcManager.AddService("beacon", &LighthouseBeaconNode{
		ExecutionNode: "el",
	})
	svcManager.AddService("validator", &LighthouseValidator{
		BeaconNode: "beacon",
	})

	geth := &OpGeth{}
	svcManager.AddService("op-geth", geth)

	externalDaRef := o.externalDA
	if o.externalDA == "" || o.externalDA == "dev" {
		svcManager.AddService("assertion-da", &AssertionDA{
			Pk:        o.assertionDAPrivateKey,
			ImageName: parsedImageNameAssertionDa,
			ImageTag:  parsedImageTagAssertionDa,
		})
		externalDaRef = Connect("assertion-da", "http")
	}

	externalBuilderRef := o.externalBuilder
	if o.externalBuilder == "" {
		// Add a new OP-Talos service and connect it to Rollup-boost
		svcManager.AddService("op-talos", &OpTalos{
			AssertionDA:    externalDaRef,
			AssexGasLimit:  o.assexGasLimit,
			OracleContract: o.oracleContract,
			ImageName:      parsedImageNameOpTalos,
			ImageTag:       parsedImageTagOpTalos,
			BlockTag:       o.opTalosBlockTag,
			GethEnode:      *geth.Enode,
		})
		externalBuilderRef = Connect("op-talos", "authrpc")
	} else {
		// Extract just the enode ID portion without the template
		enodeURL := geth.Enode.EnodeURL("op-geth", "rpc")
		enodeID := strings.Split(enodeURL, "@")[0]

		// Log clear instructions for the user
		log.Printf("External Builder configured. Sequencer EL enode: %s@<RPC_ENDPOINT>", enodeID)
	}
	externalHttpRef := Connect("op-talos", "http")

	if o.faucetUi {
		svcManager.AddService("eth-faucet", &Faucet{
			Rpc:        externalHttpRef,
			FaucetName: "",
			Symbol:     "Phylax Demo ETH",
			FaucetPk:   o.faucetPrivateKey,
		})
	}

	elNode := "rollup-boost"

	svcManager.AddService("rollup-boost", &RollupBoost{
		ELNode:  "op-geth",
		Builder: externalBuilderRef,
	})

	svcManager.AddService("op-node", &OpNode{
		L1Node:   "el",
		L1Beacon: "beacon",
		L2Node:   elNode,
	})
	svcManager.AddService("op-batcher", &OpBatcher{
		L1Node:             "el",
		L2Node:             "op-geth",
		RollupNode:         "op-node",
		MaxChannelDuration: o.batcherMaxChannelDuration,
	})
	return svcManager, nil
}

func (o *OpTalosRecipe) Output(manifest *Manifest) map[string]interface{} {
	// Just return empty map for now
	return map[string]interface{}{}
}
