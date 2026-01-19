package mac

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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

	modelName := strings.Split(args.Model, ":")[0]

	setupScript := fmt.Sprintf(`#!/bin/bash
set -e

MODEL="%s"
MODEL_NAME="%s"

echo "=== Installing Ollama if not present ==="
if ! command -v ollama &> /dev/null; then
    brew install ollama
fi

echo "=== Starting Ollama service ==="
brew services start ollama 2>/dev/null || true

echo "=== Waiting for Ollama to be ready ==="
for i in {1..30}; do
    if curl -s http://localhost:11434/api/tags | grep -q "models"; then
        echo "Ollama is ready"
        break
    fi
    echo "Waiting for Ollama... ($i/30)"
    sleep 1
done

echo "=== Checking if model exists ==="
if ollama list 2>/dev/null | grep -q "$MODEL_NAME"; then
    echo "Model $MODEL already exists"
else
    echo "Pulling model $MODEL (this may take a while)..."
    ollama pull "$MODEL"
fi

echo "=== Verifying model ==="
ollama list | grep "$MODEL_NAME"

echo "=== Ollama setup complete ==="
`, args.Model, modelName)

	// Use pulumi-command to defer execution until 'pulumi up'
	setup, err := local.NewCommand(ctx, name+"-setup", &local.CommandArgs{
		Create: pulumi.String(setupScript),
		Delete: pulumi.String("brew services stop ollama 2>/dev/null || true"),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama command: %w", err)
	}

	component.Model = pulumi.String(args.Model).ToStringOutput()
	component.Status = setup.Stdout.ApplyT(func(stdout string) string {
		if strings.Contains(stdout, modelName) {
			return "ready"
		}
		return "model not found"
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"model":  component.Model,
		"status": component.Status,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
