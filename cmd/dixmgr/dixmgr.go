package main

import (
	"fmt"
	"flag"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	dix "github.com/pierreaubert/dotidx"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configFile := flag.String("conf", "", "toml configuration file")
	templateDir := flag.String("template", "./conf/templates", "toml configuration file")
	flag.Parse()

	if configFile == nil || *configFile == "" {
		log.Fatal("Configuration file must be specified")
	}

	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	targetDir := fmt.Sprintf(`%s-%s`, config.TargetDir, config.Name)
	if err = os.Mkdir(targetDir, 0700); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	if err := generateFileFromTemplate(*config, *templateDir, targetDir); err != nil {
		log.Fatal(err)
	}
}

func generateFileFromTemplate(config dix.MgrConfig, sourceDir, destDir string) error {
	err := os.Mkdir(destDir, 0700)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("failed creating directory: %s\n", destDir)
		log.Fatal(err)
	}
	var processDir func(path string, d fs.DirEntry, err error) error

	processDir = func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, "~") || strings.HasPrefix(path, "#") {
 			fmt.Printf("Skipping backup file: %s\n", path)
 			return nil
 		}

		if ! d.IsDir() {
			if strings.HasSuffix(path, ".tmpl") {
				filename := filepath.Join(
					destDir,
					strings.TrimPrefix(strings.TrimSuffix(path, ".tmpl"),
						strings.TrimPrefix(sourceDir, "./")),
				)
				return processFileAsTemplate(path, filename, &config)
			}
			return copyFile(
				path,
				fmt.Sprintf("%s/%s",
					destDir,
					strings.TrimPrefix(
						path,
						strings.TrimPrefix(sourceDir, "./"))))
		}
		fileDir := filepath.Join(destDir, strings.TrimPrefix(d.Name(), sourceDir))
 		if err := os.MkdirAll(fileDir, 0700); err != nil {
 			return fmt.Errorf("failed to create directory %s: %w", fileDir, err)
 		}
 		// fmt.Printf("Created directory: %s\n", fileDir)
 		return nil
	}

	return filepath.WalkDir(sourceDir, processDir)
}

func processFileAsTemplate(src, dst string, config *dix.MgrConfig) error {
 	tmplData, err := os.ReadFile(src)
 	if err != nil {
 		return fmt.Errorf("failed to read template file %s: %w", src, err)
 	}

 	tmpl, err := template.New(filepath.Base(src)).Parse(string(tmplData))
 	if err != nil {
 		return fmt.Errorf("failed to parse template %s: %w", src, err)
 	}

	if _, err := os.Stat(dst) ; err == nil {
		if err := os.Chmod(dst, 0600); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
	}

  	outFile, err := os.Create(dst)
 	if err != nil {
 		return fmt.Errorf("failed to create output file %s: %w", dst, err)
 	}
 	defer outFile.Close()

 	if err := tmpl.Execute(outFile, config); err != nil {
 		return fmt.Errorf("failed to execute template %s: %w", src, err)
 	}

	if err := os.Chmod(dst, 0400); err != nil {
		return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
	}

 	fmt.Printf("Generated %s\n", dst)
 	return nil
 }

func copyFile(src, dst string) error {
 	srcFile, err := os.Open(src)
 	if err != nil {
 		return fmt.Errorf("failed to open source file %s: %w", src, err)
 	}
 	defer srcFile.Close()

	if _, err := os.Stat(dst) ; err == nil {
		if err := os.Chmod(dst, 0600); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
	}

 	dstFile, err := os.Create(dst)
 	if err != nil {
 		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
 	}
 	defer dstFile.Close()

 	_, err = io.Copy(dstFile, srcFile)
 	if err != nil {
 		return fmt.Errorf("failed to copy file contents from %s to %s: %w", src, dst, err)
 	}

	if strings.HasSuffix(dst, ".conf") || strings.HasSuffix(dst, ".json") {
		if err := os.Chmod(dst, 0400); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
	}

 	fmt.Printf("Copied    %s\n", dst)
 	return nil
 }
