package builder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

type BuildConfig struct {
	Directory string `hcl:"directory,optional"`
}

type Builder struct {
	config BuildConfig
}

func (b *Builder) Config() (interface{}, error) {
	return &b.config, nil
}

func (b *Builder) ConfigSet(config interface{}) error {
	c, ok := config.(*BuildConfig)
	if !ok {
		return fmt.Errorf("expected *BuildConfig as parameter")
	}

	if c.Directory == "" {
		return fmt.Errorf("directory must be set to a valid directory")
	}

	return nil
}

func (b *Builder) BuildFunc() interface{} {
	return b.build
}

func (b *Builder) build(ctx context.Context, ui terminal.UI) (*Binary, error) {
	u := ui.Status()
	defer func() {
		_ = u.Close()
	}()

	u.Update("Building application")

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "waypoint-build-")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Copy the source code to the temp directory
	sourceDir := "."
	if b.config.Directory != "" {
		sourceDir = b.config.Directory
	}

	if err := copyDir(sourceDir, tempDir); err != nil {
		return nil, fmt.Errorf("error copying source directory to temp directory: %w", err)
	}

	// Run npm install
	cmd := exec.CommandContext(ctx, "npm", "install")
	cmd.Dir = tempDir
	if err := runCommand(cmd, ui); err != nil {
		return nil, fmt.Errorf("error running npm install: %w", err)
	}

	// Run npm run build
	cmd = exec.CommandContext(ctx, "npm", "run", "build")
	cmd.Dir = tempDir
	if err := runCommand(cmd, ui); err != nil {
		return nil, fmt.Errorf("error running npm run build: %w", err)
	}

	// Return the path to the binary
	binaryPath := filepath.Join(tempDir, "build", "index.js")
	return &Binary{Location: binaryPath}, nil
}

// Copy the contents of src to dst recursively
func copyDir(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Copy file src to dst
func copyFile(src string, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() {
		_ = srcFile.Close()
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		_ = dstFile.Close()
	}()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// Run cmd and output the output to ui
func runCommand(cmd *exec.Cmd, ui terminal.UI) error {
	u := ui.Status()
	defer func() {
		_ = u.Close()
	}()

	cmd.Stderr = os.Stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	defer func() {
		_ = stdoutPipe.Close()
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		u.Update(scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
