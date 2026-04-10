package cli

// Shell completions are handled by Cobra's built-in completion command,
// which is automatically registered on every root command. No additional
// code generation is needed — the generated app inherits:
//
//   petstore completion bash
//   petstore completion zsh
//   petstore completion fish
//   petstore completion powershell
//
// This file exists as the anchor for T024 and future custom completion logic.
