package generator

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// Compiler compiles generated SDK code
type Compiler struct {
	workDir string
}

// NewCompiler creates a new compiler
func NewCompiler(workDir string) *Compiler {
	return &Compiler{
		workDir: workDir,
	}
}

// CompileResult holds compilation results
type CompileResult struct {
	Success bool
	Output  string
	Error   error
}

// Compile compiles a generated SDK file
func (c *Compiler) Compile(filePath string) (*CompileResult, error) {
	result := &CompileResult{}

	// Run go build on the file
	cmd := exec.Command("go", "build", filePath)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Error = fmt.Errorf("compilation failed: %w\n%s", err, output)
		return result, result.Error
	}

	result.Success = true
	return result, nil
}

// CompileBatch compiles multiple SDK files
func (c *Compiler) CompileBatch(filePaths []string) []*CompileResult {
	results := make([]*CompileResult, len(filePaths))

	for i, path := range filePaths {
		result, _ := c.Compile(path) // Error is captured in result.Error
		results[i] = result
	}

	return results
}

// Verify runs go vet on generated code
func (c *Compiler) Verify(filePath string) error {
	cmd := exec.Command("go", "vet", filePath)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verification failed: %w\n%s", err, output)
	}

	return nil
}

// Format runs gofmt on generated code
func (c *Compiler) Format(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	cmd := exec.Command("gofmt", "-w", absPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("formatting failed: %w\n%s", err, output)
	}

	return nil
}

// BuildPackage builds entire package
func (c *Compiler) BuildPackage(packagePath string) (*CompileResult, error) {
	result := &CompileResult{}

	cmd := exec.Command("go", "build", packagePath)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Error = fmt.Errorf("package build failed: %w\n%s", err, output)
		return result, result.Error
	}

	result.Success = true
	return result, nil
}
