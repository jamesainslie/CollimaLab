package mac

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// OllamaArgs contains configuration for Ollama
type OllamaArgs struct {
	Model string // e.g., "mistral-small:24b-instruct-2501-q4_K_M"
}

// Ollama represents an Ollama installation
type Ollama struct {
	pulumi.ResourceState

	Model  pulumi.StringOutput `pulumi:"model"`
	Status pulumi.StringOutput `pulumi:"status"`
}

// NewOllama installs Ollama and pulls the specified model
func NewOllama(ctx *pulumi.Context, name string, args *OllamaArgs, opts ...pulumi.ResourceOption) (*Ollama, error) {
	component := &Ollama{}
	err := ctx.RegisterComponentResource("colimalab:mac:Ollama", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Install Ollama if not present
	if !util.CommandExists("ollama") {
		if _, err := util.RunLocal("brew", "install", "ollama"); err != nil {
			return nil, fmt.Errorf("failed to install ollama: %w", err)
		}
	}

	// Start Ollama service
	// Error ignored: service may already be running or will start on first use
	_, _ = util.RunLocal("brew", "services", "start", "ollama")

	// Wait for Ollama to be ready (poll until API responds)
	for i := 0; i < 30; i++ {
		output, err := util.RunLocalShell("curl -s http://localhost:11434/api/tags")
		if err == nil && strings.Contains(output, "models") {
			break
		}
		// Error expected during startup while service initializes
		time.Sleep(time.Second)
	}

	// Check if model already exists
	// Error ignored: if list fails, assume model doesn't exist and proceed with pull
	listOutput, _ := util.RunLocalShell("ollama list")
	modelName := strings.Split(args.Model, ":")[0]
	modelExists := strings.Contains(listOutput, modelName)

	if !modelExists {
		ctx.Log.Info(fmt.Sprintf("Pulling model %s (this may take a while)...", args.Model), nil)
		if _, err := util.RunLocal("ollama", "pull", args.Model); err != nil {
			return nil, fmt.Errorf("failed to pull ollama model: %w", err)
		}
	}

	// Verify model is available
	// Error ignored: if list fails, status will be "model not found"
	listOutput, _ = util.RunLocalShell("ollama list")
	status := "ready"
	if !strings.Contains(listOutput, modelName) {
		status = "model not found"
	}

	component.Model = pulumi.String(args.Model).ToStringOutput()
	component.Status = pulumi.String(status).ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"model":  component.Model,
		"status": component.Status,
	})

	return component, nil
}
