package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GennoBou/localsend/internal/i18n"
	"github.com/GennoBou/localsend/pkg/client"
	"github.com/GennoBou/localsend/pkg/crypto"
	"github.com/GennoBou/localsend/pkg/discovery"
	"github.com/GennoBou/localsend/pkg/protocol"
	"github.com/GennoBou/localsend/pkg/server"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	langFlag          string
	debugFlag         bool
	discoveryModeFlag string
)

func logDebug(format string, v ...interface{}) {
	if debugFlag {
		fmt.Printf("[DEBUG] "+format+"\n", v...)
	}
}

var rootCmd = &cobra.Command{
	Use:   "localsend",
	Short: "LocalSend Go CLI implementation",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if langFlag != "" {
			i18n.SetLanguage(langFlag)
		}
		// Validate discovery-mode
		mode := discovery.DiscoveryMode(strings.ToLower(discoveryModeFlag))
		if mode != discovery.DiscoveryModeHybrid && mode != discovery.DiscoveryModeMulticast && mode != discovery.DiscoveryModeMDNS {
			fmt.Printf("Error: invalid discovery-mode %q. Allowed values: hybrid, multicast, mdns\n", discoveryModeFlag)
			os.Exit(1)
		}
	},
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for LocalSend devices in the network",
	Run: func(cmd *cobra.Command, args []string) {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		timeoutSec, _ := cmd.Flags().GetFloat64("timeout")

		timeout := time.Duration(timeoutSec * float64(time.Second))

		// Temporary device configuration for scanning (port 0 as we do not listen)
		myDevice := protocol.GetDefaultDevice(0, "https", false)

		if !jsonOutput {
			fmt.Println(i18n.T("scanning", nil))
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		var discoveredDevices sync.Map

		onDiscover := func(dev protocol.Device) {
			if dev.Fingerprint == myDevice.Fingerprint && dev.Port == myDevice.Port {
				return
			}
			key := fmt.Sprintf("%s:%d", dev.IP, dev.Port)
			discoveredDevices.Store(key, dev)
		}

		// Start listeners and advertising using the selected mode
		mode := discovery.DiscoveryMode(strings.ToLower(discoveryModeFlag))
		_ = discovery.StartListeners(ctx, myDevice, mode, onDiscover, func(dev protocol.Device) {})
		_, _ = discovery.StartAdvertising(ctx, myDevice, mode, true)

		// Start Legacy HTTP scan (only for hybrid or multicast mode)
		if mode == discovery.DiscoveryModeHybrid || mode == discovery.DiscoveryModeMulticast {
			go discovery.ScanLegacy(ctx, myDevice, onDiscover)
		}

		// Wait until timeout
		<-ctx.Done()

		// Build list of discovered devices
		var list []protocol.Device
		discoveredDevices.Range(func(key, value interface{}) bool {
			list = append(list, value.(protocol.Device))
			return true
		})

		if jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(list)
			return
		}

		if len(list) == 0 {
			fmt.Println(i18n.T("no_devices", nil))
			os.Exit(2)
		}

		for i, dev := range list {
			fmt.Printf("[%d] %s (%s) - %s://%s:%d\n", i+1, dev.Alias, dev.DeviceModel, dev.Protocol, dev.IP, dev.Port)
		}
	},
}

var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Start LocalSend server to wait and receive files",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		noTLS, _ := cmd.Flags().GetBool("no-tls")
		alias, _ := cmd.Flags().GetString("alias")
		tlsStrict, _ := cmd.Flags().GetBool("tls-strict")
		pin, _ := cmd.Flags().GetString("pin")
		dir, _ := cmd.Flags().GetString("dir")
		yes, _ := cmd.Flags().GetBool("yes")

		protocolStr := "https"
		if noTLS {
			protocolStr = "http"
		}

		// Automatically search for an available port by testing bind success (up to 10 attempts)
		startPort := port
		currentPort := startPort
		maxPort := startPort + 9
		var ln net.Listener
		var err error

		for {
			ln, err = net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", currentPort))
			if err == nil {
				ln.Close() // Temporarily close on successful test
				break
			}

			if currentPort < maxPort {
				fmt.Println(i18n.T("port_bind_failed", map[string]interface{}{
					"Port":     currentPort,
					"NextPort": currentPort + 1,
				}))
				currentPort++
			} else {
				fmt.Println(i18n.T("all_port_bind_failed", map[string]interface{}{
					"MaxPort": maxPort,
				}))
				os.Exit(1)
			}
		}

		port = currentPort

		myDevice := protocol.GetDefaultDevice(port, protocolStr, true)
		if alias != "" {
			myDevice.Alias = alias
		}

		saveDir := dir
		if saveDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current directory: %v\n", err)
				os.Exit(1)
			}
			saveDir = cwd
		}
		absSaveDir, err := filepath.Abs(saveDir)
		if err == nil {
			saveDir = absSaveDir
		}

		var tlsCert tls.Certificate
		if !noTLS {
			home, err := os.UserHomeDir()
			var certPath, keyPath string
			if err == nil {
				certPath = filepath.Join(home, ".config", "localsend", "cert.pem")
				keyPath = filepath.Join(home, ".config", "localsend", "key.pem")
			} else {
				certPath = filepath.Join(".", "cert.pem")
				keyPath = filepath.Join(".", "key.pem")
			}

			certInfo, err := crypto.LoadOrGenerateCredentials(certPath, keyPath)
			if err != nil {
				fmt.Printf("Failed to load/generate credentials: %v\n", err)
				os.Exit(1)
			}
			tlsCert = certInfo.TLSCert
			myDevice.Fingerprint = certInfo.Fingerprint
		} else {
			myDevice.Fingerprint = uuid.NewString()
		}

		srv := server.NewServer(myDevice, tlsCert, saveDir, pin, tlsStrict)

		srv.OnPrepareUpload = func(sender protocol.Device, files []protocol.FileMetadata) (map[string]bool, bool) {
			if yes {
				return nil, true
			}

			fmt.Println()
			fmt.Println(i18n.T("receiving_request", map[string]interface{}{
				"Alias": sender.Alias,
				"IP":    sender.IP,
			}))
			for _, f := range files {
				fmt.Printf("  - %s (%d bytes)\n", f.FileName, f.Size)
			}

			var totalSize int64
			for _, f := range files {
				totalSize += f.Size
			}

			fmt.Println()
			for {
				fmt.Print(i18n.T("confirm_receive", map[string]interface{}{
					"Count": len(files),
					"Size":  totalSize,
				}))
				var input string
				_, _ = fmt.Scanln(&input)
				input = strings.TrimSpace(strings.ToLower(input))
				if input == "y" || input == "yes" {
					return nil, true
				}
				if input == "n" || input == "no" {
					return nil, false
				}
			}
		}

		srv.OnProgress = func(sessionID string, fileID string, current int64, total int64) {
			percent := float64(current) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%d/%d bytes)", percent, current, total)
		}

		srv.OnDone = func(sessionID string, fileID string, savedPath string) {
			fmt.Println()
			fmt.Println(i18n.T("saving_to", map[string]interface{}{
				"Path": savedPath,
			}))
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		onDiscover := func(dev protocol.Device) {
			logDebug("Multicast discovery: %s (%s) - %s://%s:%d", dev.Alias, dev.DeviceModel, dev.Protocol, dev.IP, dev.Port)
		}
		onAnnounce := func(dev protocol.Device) {
			logDebug("Announce received (registration request): %s (%s) - %s://%s:%d", dev.Alias, dev.DeviceModel, dev.Protocol, dev.IP, dev.Port)
			go func() {
				_, _ = handleAnnounce(ctx, myDevice, &tlsCert, dev)
			}()
		}

		mode := discovery.DiscoveryMode(strings.ToLower(discoveryModeFlag))
		err = discovery.StartListeners(ctx, myDevice, mode, onDiscover, onAnnounce)
		if err != nil {
			logDebug("ディスカバリリスナーの起動失敗: %v", err)
		} else {
			logDebug("ディスカバリリスナーを正常に起動しました")
		}

		advertiserCloser, errAdv := discovery.StartAdvertising(ctx, myDevice, mode, false)
		if errAdv != nil {
			logDebug("ディスカバリ広告の登録失敗: %v", errAdv)
		} else {
			logDebug("ディスカバリ広告を正常に登録しました")
			defer advertiserCloser.Close()
		}

		fmt.Println(i18n.T("receiving_mode", map[string]interface{}{
			"Port":  port,
			"Alias": myDevice.Alias,
		}))

		err = srv.Start(port)
		if err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var sendCmd = &cobra.Command{
	Use:   "send [files...]",
	Short: "Send files or text to another device",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		targetAddr, _ := cmd.Flags().GetString("target")
		yes, _ := cmd.Flags().GetBool("yes")
		proxy, _ := cmd.Flags().GetString("proxy")
		insecure, _ := cmd.Flags().GetBool("insecure")
		ca, _ := cmd.Flags().GetString("ca")
		pin, _ := cmd.Flags().GetString("pin")
		browserMode, _ := cmd.Flags().GetBool("browser")
		textMsg, _ := cmd.Flags().GetString("text")

		if len(args) == 0 && textMsg == "" {
			fmt.Println("Error: specify at least one file or use --text (-t) to send a message.")
			os.Exit(1)
		}

		var files []client.SendFileSource
		if textMsg != "" {
			files = append(files, client.NewTextSendSource(textMsg))
		}

		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Printf("File not found: %s\n", arg)
				os.Exit(1)
			}
			if info.IsDir() {
				fmt.Printf("Directories are not supported directly yet: %s\n", arg)
				os.Exit(1)
			}

			filePath := arg
			hashStr, err := computeFileSha256(filePath)
			if err != nil {
				logDebug("Failed to compute sha256 for %s: %v", filePath, err)
			}

			files = append(files, client.SendFileSource{
				ID:       uuid.NewString(),
				FileName: filepath.Base(filePath),
				Size:     info.Size(),
				FileType: "application/octet-stream",
				Sha256:   hashStr,
				Open: func() (io.ReadCloser, error) {
					return os.Open(filePath)
				},
			})
		}

		// Port 0 because we do not listen
		myDevice := protocol.GetDefaultDevice(0, "https", false)

		home, err := os.UserHomeDir()
		var certPath, keyPath string
		if err == nil {
			certPath = filepath.Join(home, ".config", "localsend", "cert.pem")
			keyPath = filepath.Join(home, ".config", "localsend", "key.pem")
		} else {
			certPath = filepath.Join(".", "cert.pem")
			keyPath = filepath.Join(".", "key.pem")
		}
		certInfo, err := crypto.LoadOrGenerateCredentials(certPath, keyPath)
		var clientCert *tls.Certificate
		if err == nil {
			clientCert = &certInfo.TLSCert
			myDevice.Fingerprint = certInfo.Fingerprint
		}

		if browserMode {
			var sharedFiles []server.ShareFile
			for i, arg := range args {
				sharedFiles = append(sharedFiles, server.ShareFile{
					ID:       files[i].ID,
					Path:     arg,
					FileName: files[i].FileName,
					Size:     files[i].Size,
					FileType: "application/octet-stream",
				})
			}

			srvPort := 53318
			srvDevice := protocol.GetDefaultDevice(srvPort, "http", true)
			srv := server.NewServer(srvDevice, tls.Certificate{}, "", "", false)

			err = srv.StartDownloadServer(srvPort, sharedFiles)
			if err != nil {
				fmt.Printf("Failed to start browser server: %v\n", err)
				os.Exit(1)
			}

			localIPs, _ := discovery.GetLocalIPs()
			displayIP := "localhost"
			if len(localIPs) > 0 {
				displayIP = localIPs[0]
			}

			fmt.Println(i18n.T("download_api_started", map[string]interface{}{
				"IP":   displayIP,
				"Port": srvPort,
			}))

			select {} // Wait until browser transfer completes
		}

		var targetDevice protocol.Device

		if targetAddr != "" {
			host, portStr, err := net.SplitHostPort(targetAddr)
			if err != nil {
				host = targetAddr
				portStr = strconv.Itoa(protocol.DefaultPort)
			}
			portVal, _ := strconv.Atoi(portStr)

			targetDevice = protocol.Device{
				IP:       host,
				Port:     portVal,
				Protocol: "https",
			}

			tempCli, err := client.NewClient(myDevice, clientCert, proxy, insecure, ca)
			if err != nil {
				fmt.Printf("Failed to init client: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			partner, err := tempCli.Register(ctx, &targetDevice)
			cancel()
			if err != nil {
				fmt.Printf("Debug: HTTPS connection failed: %v. Retrying with HTTP...\n", err)
				targetDevice.Protocol = "http"
				tempCli2, _ := client.NewClient(myDevice, clientCert, proxy, insecure, ca)
				ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
				partner2, err2 := tempCli2.Register(ctx2, &targetDevice)
				cancel2()
				if err2 != nil {
					fmt.Printf("Failed to connect to %s: %v (HTTP Error: %v)\n", targetAddr, err2, err2)
					os.Exit(2)
				}
				targetDevice = *partner2
			} else {
				targetDevice = *partner
			}
		} else {
			fmt.Println(i18n.T("scanning", nil))

			ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
			var discoveredDevices sync.Map

			onDiscover := func(dev protocol.Device) {
				if dev.Fingerprint == myDevice.Fingerprint && dev.Port == myDevice.Port {
					return
				}
				key := fmt.Sprintf("%s:%d", dev.IP, dev.Port)
				discoveredDevices.Store(key, dev)
			}

			mode := discovery.DiscoveryMode(strings.ToLower(discoveryModeFlag))
			_ = discovery.StartListeners(ctx, myDevice, mode, onDiscover, func(dev protocol.Device) {})
			_, _ = discovery.StartAdvertising(ctx, myDevice, mode, true)
			// Start Legacy HTTP scan (only for hybrid or multicast mode)
			if mode == discovery.DiscoveryModeHybrid || mode == discovery.DiscoveryModeMulticast {
				go discovery.ScanLegacy(ctx, myDevice, onDiscover)
			}

			<-ctx.Done()
			cancel()

			var list []protocol.Device
			discoveredDevices.Range(func(key, value interface{}) bool {
				list = append(list, value.(protocol.Device))
				return true
			})

			if len(list) == 0 {
				fmt.Println(i18n.T("no_devices", nil))
				os.Exit(2)
			}

			fmt.Println()
			fmt.Println(i18n.T("select_device", nil))
			for i, dev := range list {
				fmt.Printf("[%d] %s (%s) - %s://%s:%d\n", i+1, dev.Alias, dev.DeviceModel, dev.Protocol, dev.IP, dev.Port)
			}

			var choice int
			for {
				fmt.Print("Enter number: ")
				var input string
				_, _ = fmt.Scanln(&input)
				choice, err = strconv.Atoi(strings.TrimSpace(input))
				if err == nil && choice >= 1 && choice <= len(list) {
					break
				}
				fmt.Println("Invalid choice. Please select from the list.")
			}

			targetDevice = list[choice-1]
		}

		if !yes {
			var totalSize int64
			for _, f := range files {
				totalSize += f.Size
			}
			fmt.Println()
			fmt.Printf("Target: %s (%s)\n", targetDevice.Alias, targetDevice.DeviceModel)
			fmt.Printf("Files: %d files (%d bytes)\n", len(files), totalSize)
			for {
				fmt.Print("Do you want to send? (y/n): ")
				var input string
				_, _ = fmt.Scanln(&input)
				input = strings.TrimSpace(strings.ToLower(input))
				if input == "y" || input == "yes" {
					break
				}
				if input == "n" || input == "no" {
					os.Exit(0)
				}
			}
		}

		cli, err := client.NewClient(myDevice, clientCert, proxy, insecure, ca)
		if err != nil {
			fmt.Printf("Failed to initialize client: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println(i18n.T("sending_files", map[string]interface{}{
			"Alias": targetDevice.Alias,
		}))

		progressFunc := func(fileID string, sentBytes int64) {
			var fileMeta client.SendFileSource
			for _, f := range files {
				if f.ID == fileID {
					fileMeta = f
					break
				}
			}
			percent := float64(sentBytes) / float64(fileMeta.Size) * 100
			fmt.Printf("\rProgress for %s: %.1f%% (%d/%d bytes)", fileMeta.FileName, percent, sentBytes, fileMeta.Size)
		}

		ctx := context.Background()
		results, err := cli.SendFiles(ctx, &targetDevice, files, pin, progressFunc)
		fmt.Println()

		if err != nil {
			if err == protocol.ErrRejected {
				fmt.Println(i18n.T("transfer_rejected", nil))
				os.Exit(3)
			}
			fmt.Println(i18n.T("transfer_failed", map[string]interface{}{
				"Error": err.Error(),
			}))
			os.Exit(4)
		}

		hasError := false
		for _, r := range results {
			if r.Err != nil {
				var fileName string
				for _, f := range files {
					if f.ID == r.FileID {
						fileName = f.FileName
						break
					}
				}
				fmt.Printf("File %s failed to send: %v\n", fileName, r.Err)
				hasError = true
			}
		}

		if hasError {
			os.Exit(4)
		}

		fmt.Println(i18n.T("transfer_complete", nil))
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", "Language to use (en, ja)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&discoveryModeFlag, "discovery-mode", "hybrid", "Discovery mode (hybrid, multicast, mdns)")

	scanCmd.Flags().Bool("json", false, "Output results in JSON format")
	scanCmd.Flags().Float64("timeout", 2.5, "Scan timeout in seconds")
	rootCmd.AddCommand(scanCmd)

	receiveCmd.Flags().Int("port", protocol.DefaultPort, "Port to listen on")
	receiveCmd.Flags().Bool("no-tls", false, "Disable TLS (use HTTP)")
	receiveCmd.Flags().String("alias", "", "Device alias override")
	receiveCmd.Flags().Bool("tls-strict", false, "Strict client certificate validation (mTLS)")
	receiveCmd.Flags().String("pin", "", "PIN code required to receive files")
	receiveCmd.Flags().String("dir", "", "Directory to save received files")
	receiveCmd.Flags().Bool("yes", false, "Accept transfer requests automatically")
	rootCmd.AddCommand(receiveCmd)

	sendCmd.Flags().String("target", "", "Target device address (IP:Port)")
	sendCmd.Flags().Bool("yes", false, "Skip confirmation prompts")
	sendCmd.Flags().String("proxy", "", "Proxy server URL")
	sendCmd.Flags().BoolP("insecure", "k", false, "Skip TLS certificate validation")
	sendCmd.Flags().String("ca", "", "Path to custom CA certificate file")
	sendCmd.Flags().String("pin", "", "PIN code required by target device")
	sendCmd.Flags().Bool("browser", false, "Start reverse transfer in browser mode")
	sendCmd.Flags().StringP("text", "t", "", "Send a text message instead of or in addition to files")
	rootCmd.AddCommand(sendCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Command execution failed: %v\n", err)
		os.Exit(1)
	}
}

// computeFileSha256 computes the SHA-256 checksum of a file given its path.
func computeFileSha256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// handleAnnounce attempts to register back with a discovered device upon receiving an announce signal.
func handleAnnounce(ctx context.Context, myDevice protocol.Device, tlsCert *tls.Certificate, dev protocol.Device) (*protocol.Device, error) {
	cli, err := client.NewClient(myDevice, tlsCert, "", true, "")
	if err != nil {
		logDebug("登録用クライアントの初期化失敗: %v", err)
		return nil, fmt.Errorf("failed to initialize registration client: %w", err)
	}
	partner, err := cli.Register(ctx, &dev)
	if err != nil {
		logDebug("%s への HTTP 登録（対向登録）リクエスト失敗: %v", dev.IP, err)
		return nil, fmt.Errorf("registration request to %s failed: %w", dev.IP, err)
	}
	logDebug("HTTP 登録成功: %s に登録されました (相手別名: %s)", dev.IP, partner.Alias)
	return partner, nil
}
