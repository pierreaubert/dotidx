package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pierreaubert/dotidx/dix"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configFile := flag.String("conf", "", "toml configuration file")
	templatesDir := flag.String("templates", "./conf/templates", "templated configuration files")
	scriptsDir := flag.String("scripts", "./conf/scripts", "templated script files")
	appDir := flag.String("app", "./app", "templated script files")
	flag.Parse()

	if configFile == nil || *configFile == "" {
		log.Fatal("Configuration file must be specified")
	}

	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if errs := checkConfig(*config); len(errs) > 0 {
		for i := range errs {
			log.Printf("%s", errs[i])
		}
		log.Fatal("Config validation failed!")
	}

	dirs := []string{
		config.DotidxRoot,
		config.DotidxBin,
		config.DotidxLogs,
		config.DotidxRun,
		config.DotidxRuntime,
		config.DotidxBackup,
		config.DotidxStatic,
	}
	for i := range dirs {
		if err = os.Mkdir(dirs[i], 0700); err != nil && !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	targetDir := fmt.Sprintf(`%s-%s`, config.TargetDir, config.Name)
	if err = os.Mkdir(targetDir, 0700); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	if err := generateFileFromTemplate(*config, *templatesDir, targetDir); err != nil {
		log.Fatal(err)
	}

	if err := generateFilePerRelaychain(*config, *templatesDir, targetDir); err != nil {
		log.Fatal(err)
	}

	if err := generateFilePerChain(*config, *templatesDir, targetDir); err != nil {
		log.Fatal(err)
	}

	if err = os.Mkdir(config.DotidxBin, 0700); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	if err := generateScriptsFromTemplate(*config, *scriptsDir, config.DotidxBin); err != nil {
		log.Fatal(err)
	}

	if err := copyFile(*configFile, fmt.Sprintf("%s/%s", targetDir, filepath.Base(*configFile))); err != nil {
		log.Fatal(err)
	}

	if err := copyStaticWebsite(*config, *appDir); err != nil {
		log.Fatal(err)
	}

}

func copyStaticWebsite(config dix.MgrConfig, appDir string) error {

	processDir := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return copyFile(path, config.DotidxStatic+"/"+filepath.Base(path))
		}
		return nil
	}

	return filepath.WalkDir(appDir, processDir)
}

func checkConfigPortCollision(config dix.MgrConfig) error {
	ports := make(map[int]bool, 0)

	add := func(port int) error {
		if ports[port] == true {
			return fmt.Errorf("port %d is already in use", port)
		}
		ports[port] = true
		return nil
	}

	if err := add(config.DotidxDB.Port); err != nil {
		return err
	}
	if err := add(config.DotidxFE.Port); err != nil {
		return err
	}

	for relay := range config.Parachains {
		for chain := range config.Parachains[relay] {
			chainConfig := config.Parachains[relay][chain]
			if err := add(chainConfig.PortRPC); err != nil {
				return err
			}
			if err := add(chainConfig.PortWS); err != nil {
				return err
			}
			if err := add(chainConfig.ChainreaderPort); err != nil {
				return err
			}
			if err := add(chainConfig.PrometheusPort); err != nil {
				return err
			}
			for i := range chainConfig.SidecarCount {
				if err := add(chainConfig.SidecarPort + 1 + i); err != nil {
					return err
				}
				if err := add(chainConfig.SidecarPrometheusPort + 1 + i); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkConfig(config dix.MgrConfig) []error {
	errs := make([]error, 0)

	if err := checkConfigPortCollision(config); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func toTitle(s string) string {
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func generateFilePerChain(config dix.MgrConfig, sourceDir, destDir string) error {

	if err := generateNodeFilePerChain(config, sourceDir, destDir); err != nil {
		return err
	}

	if err := generateSidecarFilePerChain(config, sourceDir, destDir); err != nil {
		return err
	}

	return nil
}

func generateSidecarFilePerChain(config dix.MgrConfig, sourceDir, destDir string) error {
	confDir := fmt.Sprintf(`%s/conf`, destDir)
	err := os.Mkdir(confDir, 0700)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("failed creating directory: %s\n", confDir)
		log.Fatal(err)
	}

	for relay := range config.Parachains {
		for chain := range config.Parachains[relay] {
			for i := range config.Parachains[relay][chain].SidecarCount {
				dst := fmt.Sprintf(`%s/conf/%s-%s-%d-sidecar.conf`,
					destDir,
					relay,
					chain,
					i+1,
				)
				if _, err := os.Stat(dst); err == nil {
					if err := os.Chmod(dst, 0600); err != nil {
						return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
					}
				}
				outFile, err := os.Create(dst)
				if err != nil {
					return fmt.Errorf("failed to create output file %s: %w", dst, err)
				}
				defer outFile.Close()

				port := config.Parachains[relay][chain].SidecarPort + 1 + i
				prom_port := config.Parachains[relay][chain].SidecarPrometheusPort + 1 + i
				ip := config.Parachains[relay][chain].NodeIP
				if relay == chain {
					ip = config.Parachains[relay][chain].RelayIP
				}
				nodeTmpl := fmt.Sprintf(`
# configuration for a sidecar per chain
SAS_METRICS_ENABLED=true
SAS_METRICS_PROM_HOST="{{.Parachains.%[2]s.%[4]s.SidecarIP}}"
SAS_METRICS_PROM_PORT=%[6]d
SAS_METRICS_LOKI_HOST="127.0.0.1"
SAS_METRICS_LOKI_PORT=3100
SAS_WRITE_PATH="{{.DotidxLogs}}"
SAS_SUBSTRATE_URL="ws://%[7]s:{{.Parachains.%[2]s.%[4]s.PortRPC}}"
SAS_EXPRESS_BIND_HOST="{{.Parachains.%[2]s.%[4]s.SidecarIP}}"
SAS_EXPRESS_PORT=%[5]d
`, toTitle(relay), relay, toTitle(chain), chain, port, prom_port, ip)

				// log.Printf(nodeTmpl)
				node, err := template.New("node").Parse(nodeTmpl)
				if err != nil {
					return fmt.Errorf("failed to parse template relay: %w", err)
				}
				if err := node.Execute(outFile, config); err != nil {
					return fmt.Errorf("failed to execute template relay: %w", err)
				}
				if err := os.Chmod(dst, 0400); err != nil {
					return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
				}
				fmt.Printf("Generated %s\n", dst)
			}
		}
	}
	return nil
}

func generateNodeFilePerChain(config dix.MgrConfig, sourceDir, destDir string) error {
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
			if _, err := os.Stat(dst); err == nil {
				if err := os.Chmod(dst, 0600); err != nil {
					return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
				}
			}
			outFile, err := os.Create(dst)
			if err != nil {
				return fmt.Errorf("failed to create output file %s: %w", dst, err)
			}
			defer outFile.Close()

			nodeTmpl := fmt.Sprintf(`
# configuration for the relay chain
NODE_BIN={{.Parachains.%[2]s.%[4]s.Bin}}
NODE_CHAIN={{.Parachains.%[2]s.%[4]s.Name}}
NODE_NAME=10%[1]s%[3]s
NODE_BASE_PATH={{.Parachains.%[2]s.%[4]s.Basepath}}
NODE_PORT_WS={{.Parachains.%[2]s.%[4]s.PortWS}}
NODE_PORT_RPC={{.Parachains.%[2]s.%[4]s.PortRPC}}
NODE_RELAY="ws://{{.Parachains.%[2]s.%[2]s.RelayIP}}:{{.Parachains.%[2]s.%[2]s.PortRPC}}"
NODE_PROM_PORT={{.Parachains.%[2]s.%[4]s.PrometheusPort}}
`, toTitle(relay), relay, toTitle(chain), chain)

			if config.Parachains[relay][chain].BootNodes != "" {
				bootnodesTmpl := fmt.Sprintf("BOOTNODES=%s\n", config.Parachains[relay][chain].BootNodes)
				nodeTmpl = fmt.Sprintf("%s%s", nodeTmpl, bootnodesTmpl)
			}

			// log.Printf(nodeTmpl)
			node, err := template.New("node").Parse(nodeTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse template relay: %w", err)
			}
			if err := node.Execute(outFile, config); err != nil {
				return fmt.Errorf("failed to execute template relay: %w", err)
			}
			if err := os.Chmod(dst, 0400); err != nil {
				return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
			}
			fmt.Printf("Generated %s\n", dst)
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
		if _, err := os.Stat(dst); err == nil {
			if err := os.Chmod(dst, 0600); err != nil {
				return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
			}
		}
		outFile, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", dst, err)
		}
		defer outFile.Close()

		relayTmpl := fmt.Sprintf(`
# configuration for the relay chain
NODE_BIN={{.Parachains.%[2]s.%[2]s.Bin}}
NODE_CHAIN=%[2]s
NODE_NAME=10%[1]s
NODE_BASE_PATH={{.Parachains.%[2]s.%[2]s.Basepath}}
NODE_PORT_WS={{.Parachains.%[2]s.%[2]s.PortWS}}
NODE_PORT_RPC={{.Parachains.%[2]s.%[2]s.PortRPC}}
NODE_PROM_PORT={{.Parachains.%[2]s.%[2]s.PrometheusPort}}
`, toTitle(relay), relay)

		// log.Printf(relayTmpl)
		relay, err := template.New("relay").Parse(relayTmpl)
		if err != nil {
			return fmt.Errorf("failed to parse template relay: %w", err)
		}
		if err := relay.Execute(outFile, config); err != nil {
			return fmt.Errorf("failed to execute template relay: %w", err)
		}
		if err := os.Chmod(dst, 0400); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
		fmt.Printf("Generated %s\n", dst)
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
		if !d.IsDir() {
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

func generateScriptsFromTemplate(config dix.MgrConfig, sourceDir, destDir string) error {
	err := os.Mkdir(destDir, 0700)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("failed creating directory: %s\n", destDir)
		log.Fatal(err)
	}
	processDir := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".tmpl") {
			filename := filepath.Join(
				destDir,
				filepath.Base(strings.TrimSuffix(path, ".tmpl")),
			)
			return processFileAsTemplate(path, filename, &config)
		}

		return nil
	}
	return filepath.WalkDir(sourceDir, processDir)
}

func processFileAsTemplate(src, dst string, config *dix.MgrConfig) error {

	if strings.HasSuffix(dst, "~") || strings.HasSuffix(dst, "#") || strings.HasPrefix(dst, ".#") {
		fmt.Printf("Skipping backup file: %s\n", dst)
		return nil
	}

	tmplData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", src, err)
	}

	tmpl, err := template.New(filepath.Base(src)).Parse(string(tmplData))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", src, err)
	}

	if _, err := os.Stat(dst); err == nil {
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

	if strings.HasSuffix(dst, ".sh") {
		if err := os.Chmod(dst, 0500); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
	} else {
		if err := os.Chmod(dst, 0400); err != nil {
			return fmt.Errorf("failed to change permissions on %s: %w", dst, err)
		}
	}

	fmt.Printf("Generated %s\n", dst)
	return nil
}

func copyFile(src, dst string) error {

	if strings.HasSuffix(dst, "~") || strings.HasSuffix(dst, "#") || strings.HasPrefix(dst, ".#") {
		fmt.Printf("Skipping backup file: %s\n", dst)
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	if _, err := os.Stat(dst); err == nil {
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
