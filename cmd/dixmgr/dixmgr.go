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


	if err := generateFilePerRelaychain(*config, *templateDir, targetDir); err != nil {
		log.Fatal(err)
	}

	if err := generateFilePerChain(*config, *templateDir, targetDir); err != nil {
		log.Fatal(err)
	}
}

func toTitle(s string) string {
    return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func generateFilePerChain(config dix.MgrConfig, sourceDir, destDir string) error {
	confDir := fmt.Sprintf(`%s/conf`, destDir)
	err := os.Mkdir(confDir, 0700)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("failed creating directory: %s\n", confDir)
		log.Fatal(err)
	}

	for relay := range config.Parachains {
		for chain := range config.Parachains[relay] {
			dst := fmt.Sprintf(`%s/conf/%s-%s-node-archive.conf`,
				destDir,
				relay,
				chain,
			)
  			outFile, err := os.Create(dst)
 			if err != nil {
 				return fmt.Errorf("failed to create output file %s: %w", dst, err)
 			}
 			defer outFile.Close()

			nodeTmpl := fmt.Sprintf(`
# configuration for the relay chain
NODE_BIN={{.DotidxBin}}/polkadot-parachain
NODE_CHAIN={{.Parachains.%[2]s.%[4]s.Name}}
NODE_NAME=10%[1]s%[3]s
NODE_BASE_PATH={{.Parachains.%[2]s.%[4]s.Basepath}}
NODE_PORT_WS={{.Parachains.%[2]s.%[4]s.PortWS}}
NODE_PORT_RPC={{.Parachains.%[2]s.%[4]s.PortRPC}}
NODE_RELAY="ws://{{.Parachains.%[2]s.%[2]s.RelayIP}}:{{.Parachains.%[2]s.%[2]s.PortWS}}"
NODE_PROM_PORT={{.Parachains.%[2]s.%[4]s.PrometheusPort}}
`, toTitle(relay), relay, toTitle(chain), chain);

			log.Printf(nodeTmpl)
 			node, err := template.New("node").Parse(nodeTmpl)
 			if err != nil {
 				return fmt.Errorf("failed to parse template relay: %w", err)
 			}
 			if err := node.Execute(outFile, config); err != nil {
 				return fmt.Errorf("failed to execute template relay: %w", err)
 			}
		}
	}
	return nil
}

func generateFilePerRelaychain(config dix.MgrConfig, sourceDir, destDir string) error {
	confDir := fmt.Sprintf(`%s/conf`, destDir)
	err := os.Mkdir(confDir, 0700)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("failed creating directory: %s\n", confDir)
		log.Fatal(err)
	}

	for relay := range config.Parachains {
		dst := fmt.Sprintf(`%s/conf/%s-relay-archive.conf`,
			destDir,
			relay,
		)
  		outFile, err := os.Create(dst)
 		if err != nil {
 			return fmt.Errorf("failed to create output file %s: %w", dst, err)
 		}
 		defer outFile.Close()

		relayTmpl := fmt.Sprintf(`
# configuration for the relay chain
NODE_BIN={{.DotidxBin}}/polkadot
NODE_CHAIN=%[2]s
NODE_NAME=10%[1]s
NODE_BASE_PATH={{.Parachains.%[2]s.%[2]s.Basepath}}
NODE_PORT_WS={{.Parachains.%[2]s.%[2]s.PortWS}}
NODE_PORT_RPC={{.Parachains.%[2]s.%[2]s.PortRPC}}
NODE_PROM_PORT={{.Parachains.%[2]s.%[2]s.PrometheusPort}}
`, toTitle(relay), relay);

		// log.Printf(relayTmpl)
 		relay, err := template.New("relay").Parse(relayTmpl)
 		if err != nil {
 			return fmt.Errorf("failed to parse template relay: %w", err)
 		}
 		if err := relay.Execute(outFile, config); err != nil {
 			return fmt.Errorf("failed to execute template relay: %w", err)
 		}
	}
	return nil
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
