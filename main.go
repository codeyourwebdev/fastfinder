// #cgo !yara_no_pkg_config,!yara_static  pkg-config: yara
// #cgo !yara_no_pkg_config,yara_static   pkg-config: --static yara
// #cgo yara_no_pkg_config                LDFLAGS:    -lyara
// compile: go build -trimpath -tags yara_static -a -ldflags '-s -w -extldflags "-static"' .
// suggestion: reduce binary size with "upx --best --lzma .\fastfinder.exe"

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/dlclark/regexp2"
	"github.com/fsnotify/fsnotify"
	"github.com/hillu/go-yara/v4"
)

const FASTFINDER_VERSION = "2.0.0b"
const YARA_VERSION = "4.1.3"
const BUILDER_RC4_KEY = ">Õ°ªKb{¡§ÌB$lMÕ±9l.tòÑé¦Ø¿"

func main() {
	// parse configuration file
	parser := argparse.NewParser("fastfinder", "Incident Response - Fast suspicious file finder")
	pConfigPath := parser.String("c", "configuration", &argparse.Options{Required: false, Default: "", Help: "Fastfind configuration file"})
	pSfxPath := parser.String("b", "build", &argparse.Options{Required: false, Help: "Output a standalone package with configuration and rules in a single binary"})
	pOutLogPath := parser.String("o", "output", &argparse.Options{Required: false, Help: "Save fastfinder logs in the specified file"})
	pHideWindow := parser.Flag("n", "no-window", &argparse.Options{Required: false, Help: "Hide fastfinder window"})
	pDisableAdvUI := parser.Flag("u", "no-userinterface", &argparse.Options{Required: false, Help: "Hide advanced user interface"})
	pLogVerbosity := parser.Int("v", "verbosity", &argparse.Options{Required: false, Default: 3, Help: "File log verbosity \n\t\t\t\t | 4: Only alert\n\t\t\t\t | 3: Alert and errors\n\t\t\t\t | 2: Alerts,errors and I/O operations\n\t\t\t\t | 1: Full verbosity)\n\t\t\t\t"})
	pTriage := parser.Flag("t", "triage", &argparse.Options{Required: false, Default: false, Help: "Triage mode (infinite run - scan every new file in the input path directories)"})

	// handle argument parsing error
	err := parser.Parse(os.Args)
	if err != nil {
		log.Fatal(parser.Usage(err))
	}

	// enable advanced UI
	if *pTriage || *pDisableAdvUI || *pHideWindow || len(*pSfxPath) > 0 {
		UIactive = false
	} else {
		InitUI()
	}

	// display open file dialog when config file empty
	if len(*pConfigPath) == 0 {
		InitUI()
		OpenFileDialog()
		*pConfigPath = UIselectedConfigPath
	}

	// check for log path validity
	if len(*pOutLogPath) > 0 {
		if strings.Contains(*pOutLogPath, " ") {
			LogFatal("Log file path cannot contain spaces")
		}
	}

	// init progressbar object
	EnableProgressbar(*pDisableAdvUI)

	// configuration parsing
	var config Configuration
	config.getConfiguration(*pConfigPath)
	if config.Output.FilesCopyPath != "" {
		config.Output.FilesCopyPath = "./"
	}

	// window hidden
	if *pHideWindow && len(*pSfxPath) == 0 {
		HideConsoleWindow()
	}

	// output log to file
	if len(*pOutLogPath) > 0 && len(*pSfxPath) == 0 {
		loggingPath = *pOutLogPath
	}

	// file logging verbosity
	if *pLogVerbosity >= 1 && *pLogVerbosity <= 4 {
		loggingVerbosity = *pLogVerbosity
	}

	// run app
	if UIactive {
		go MainFastfinderRoutine(config, *pConfigPath, *pDisableAdvUI, *pHideWindow, *pSfxPath, *pTriage, *pOutLogPath, *pLogVerbosity)
		MainWindow()
	} else {
		LogMessage(LOG_INFO, LineBreak+"================================================"+LineBreak+RenderFastfinderLogo()+"================================================"+LineBreak)
		MainFastfinderRoutine(config, *pConfigPath, *pDisableAdvUI, *pHideWindow, *pSfxPath, *pTriage, *pOutLogPath, *pLogVerbosity)
	}

}

// MainFastfinderRoutine is used in every scan routine and based on config file directives
func MainFastfinderRoutine(config Configuration, pConfigPath string, pNoAdvUI bool, pHideWindow bool, pSfxPath string, pTriage bool, pOutLogPath string, pLoglevel int) {
	var rules *yara.Rules

	// check for input configuration
	if len(config.Input.Path) == 0 && len(config.Input.Content.Grep) == 0 && len(config.Input.Content.Checksum) == 0 && len(config.Input.Content.Yara) == 0 {
		LogMessage(LOG_ERROR, "(ERROR)", "Input parameters empty - cannot find any item")
		ExitProgram(1, !UIactive)
	}

	// sfx building option
	if len(pSfxPath) > 0 {
		BuildSFX(config, pSfxPath, pLoglevel, pOutLogPath, pNoAdvUI, pHideWindow)
		LogMessage(LOG_INFO, "(INFO)", "Fastfinder package generated successfully at", pSfxPath)
		ExitProgram(0, !UIactive)
	}

	// fastfinder init
	FastFinderInit(config, pConfigPath, pSfxPath, pHideWindow)

	// if yara rules mentionned - compile them
	if len(config.Input.Content.Yara) > 0 {
		rules = CompileYaraRules(config.Input.Content.Yara, config.AdvancedParameters.YaraRC4Key)
	}

	// drives enumeration
	baseDrives, excludedPaths := DriveEnumeration(config)

	// triage mode start
	if pTriage {
		if len(config.Input.Path) == 0 {
			LogFatal("No initial path specified to look for in triage mode")
		}

		if len(config.Input.Content.Yara) == 0 && len(config.Input.Content.Checksum) == 0 && len(config.Input.Content.Grep) == 0 {
			LogFatal("No criteria to look for in triage mode")
		}

		if config.Options.FindInNetworkDrives || config.Options.FindInCDRomDrives {
			LogMessage(LOG_ERROR, "(WARNING)", "Triage mode cannot retrieve files modification on network and CD-ROM drives")
			time.Sleep(3 * time.Second)
		}

		EnableProgressbar(false)

		if !pNoAdvUI {
			UIactive = false
			LogMessage(LOG_INFO, "(INFO)", "Advanced UI and progressbar disabled for performance enhancements under triage")
		}

		LogMessage(LOG_INFO, "(INFO)", "TRIAGE MODE - Use Ctrl+C to stop fastfinder")
		time.Sleep(3 * time.Second)
		InitTriageScan(config, rules, baseDrives, excludedPaths)
	}

	// start main routine
	for _, basePath := range baseDrives {
		LogMessage(LOG_VERBOSE, "(INFO)", "Enumerating files in", basePath)
		var matchContent []string
		var matchPathPattern []string

		// files listing
		filesEnumeration := ListFilesRecursively(basePath, excludedPaths)
		if runtime.GOOS != "windows" {
			excludedPaths = append(excludedPaths, basePath)
		}

		// check for files matching path patterns
		if len(config.Input.Path) > 0 {
			LogMessage(LOG_VERBOSE, "(INFO)", "Checking for paths matchs in", basePath)
			var pathRegexPatterns []*regexp2.Regexp
			for _, pattern := range config.Input.Path {
				re := regexp2.MustCompile(pattern, regexp2.IgnoreCase)
				pathRegexPatterns = append(pathRegexPatterns, re)
			}
			matchPathPattern = *PathsFinder(filesEnumeration, pathRegexPatterns)
			if !config.Options.ContentMatchDependsOnPathMatch {
				for i := 0; i < len(matchPathPattern); i++ {
					LogMessage(LOG_ALERT, "(ALERT)", "File path match on:", matchPathPattern[i])
				}
			}
		}

		// check for file matching content, checksum and yara rules
		if len(config.Input.Content.Grep) > 0 || len(config.Input.Content.Checksum) > 0 || len(config.Input.Content.Yara) > 0 {
			LogMessage(LOG_VERBOSE, "(INFO)", "Checking for content, checksum and YARA rules matchs in", basePath)

			if config.Options.ContentMatchDependsOnPathMatch && len(config.Input.Path) > 0 {
				if len(matchPathPattern) == 0 {
					LogMessage(LOG_VERBOSE, "(INFO)", "Neither path nor pattern match. no file to scan with YARA.", basePath)
				} else {
					matchContent = *FindInFilesContent(&matchPathPattern, config.Input.Content.Grep, rules, config.Input.Content.Checksum, false, config.AdvancedParameters.MaxScanFilesize, config.AdvancedParameters.CleanMemoryIfFileGreaterThanSize)
				}
			} else {
				matchContent = *FindInFilesContent(filesEnumeration, config.Input.Content.Grep, rules, config.Input.Content.Checksum, false, config.AdvancedParameters.MaxScanFilesize, config.AdvancedParameters.CleanMemoryIfFileGreaterThanSize)
			}
		}

		// listing and copy matching files
		LogMessage(LOG_INFO, "(INFO)", "scan finished in", basePath)
		if (len(matchPathPattern) > 0 && !config.Options.ContentMatchDependsOnPathMatch) || len(matchContent) > 0 {
			LogMessage(LOG_ALERT, "(INFO)", "Matching files: ")
			// output pattern matchs
			if !config.Options.ContentMatchDependsOnPathMatch {
				for i := 0; i < len(matchPathPattern); i++ {
					LogMessage(LOG_ALERT, " |", matchContent[i])
				}
			}

			// output content, checksum and yara match
			for i := 0; i < len(matchContent); i++ {
				LogMessage(LOG_ALERT, " |", matchContent[i])
			}

			// copy file matchs
			if config.Output.CopyMatchingFiles {
				LogMessage(LOG_INFO, "(INFO)", "Copy all matching files")
				if !config.Options.ContentMatchDependsOnPathMatch {
					InitProgressbar(int64(len(matchPathPattern)) + int64(len(matchContent)))
					for i := 0; i < len(matchPathPattern); i++ {
						ProgressBarStep()
						FileCopy(matchPathPattern[i], config.Output.FilesCopyPath, config.Output.Base64Files)
					}
				} else {
					InitProgressbar(int64(len(matchContent)))
				}

				for i := 0; i < len(matchContent); i++ {
					ProgressBarStep()
					FileCopy(matchContent[i], config.Output.FilesCopyPath, config.Output.Base64Files)
				}
			}
		} else {
			LogMessage(LOG_INFO, "(INFO)", "No match found")
		}
	}

	ExitProgram(0, !UIactive)
}

// FastFinderInit return basic host informations / check for mutex and return current user permissions
func FastFinderInit(config Configuration, pConfigPath string, pSfxPath string, pHideWindow bool) {
	var err error

	LogMessage(LOG_INFO, "(INIT)", "Fastfinder v"+FASTFINDER_VERSION+" with embedded YARA v"+YARA_VERSION)
	LogMessage(LOG_INFO, "(INIT)", "OS:", runtime.GOOS, "Arch:", runtime.GOARCH)
	LogMessage(LOG_INFO, "(INIT)", "Hostname:", GetHostname())
	LogMessage(LOG_INFO, "(INIT)", "User:", GetUsername())
	LogMessage(LOG_INFO, "(INIT)", "Current directory:", GetCurrentDirectory())
	LogMessage(LOG_INFO, "(INIT)", "Max file size scan:", fmt.Sprintf("%dMB", config.AdvancedParameters.MaxScanFilesize))
	LogMessage(LOG_INFO, "(INIT)", "Config file:", pConfigPath)
	LogMessage(LOG_INFO, "(INIT)", "Fastfinder executable SHA256 checksum:", FileSHA256Sum(os.Args[0]))
	LogMessage(LOG_INFO, "(INIT)", "Configuration file SHA256 checksum:", FileSHA256Sum(pConfigPath))

	if len(pSfxPath) == 0 {
		// create mutex
		if _, err = CreateMutex("fastfinder"); err != nil {
			LogMessage(LOG_ERROR, "(ERROR)", "Only one instance or fastfinder can be launched:", err.Error())
			ExitProgram(1, !UIactive)
		}

		// Retrieve current user permissions
		admin, elevated := CheckCurrentUserPermissions()
		if !admin && !elevated {
			LogMessage(LOG_ERROR, "(WARNING) fastfinder is not running with fully elevated righs. Notice that the analysis will be partial and limited to the current user scope")
			if !pHideWindow {
				time.Sleep(3 * time.Second)
			}
		}
	}
}

// DriveEnumeration enumerate drives based on configuration parameters
func DriveEnumeration(config Configuration) ([]string, []string) {
	LogMessage(LOG_VERBOSE, "(INIT)", "Enumerating drives")
	var basePaths []string
	drives, excludedPaths := EnumLogicalDrives()

	if len(drives) == 0 {
		LogMessage(LOG_ERROR, "(ERROR)", "Unable to find drives")
		ExitProgram(1, !UIactive)
	}

	for _, drive := range drives {
		if (drive.Type == DRIVE_REMOVABLE && config.Options.FindInRemovableDrives) ||
			(drive.Type == DRIVE_FIXED && config.Options.FindInHardDrives) ||
			(drive.Type == DRIVE_REMOTE && config.Options.FindInNetworkDrives) ||
			(drive.Type == DRIVE_CDROM && config.Options.FindInCDRomDrives) {
			if runtime.GOOS == "windows" || len(basePaths) == 0 {
				basePaths = append(basePaths, drive.Name)
			} else {
				alreadyParsed := false
				for _, p := range basePaths {
					if len(drive.Name) > len(p) && !strings.HasPrefix(drive.Name, p) {
						alreadyParsed = true
					}
				}
				if !alreadyParsed {
					basePaths = append(basePaths, drive.Name)
				}
			}
		} else {
			if runtime.GOOS != "windows" {
				excludedPaths = append(excludedPaths, drive.Name)
			}
		}
	}

	if len(basePaths) == 0 {
		LogMessage(LOG_ERROR, "(ERROR)", "No drive corresponding to your configuration drive type")
		ExitProgram(1, !UIactive)
	} else {
		LogMessage(LOG_VERBOSE, "(INIT)", "Looking for the following drives:")
		for _, p := range basePaths {
			LogMessage(LOG_INFO, " |", p)
		}
	}

	if len(excludedPaths) > 0 {
		LogMessage(LOG_VERBOSE, "(INFO)", "Excluding the following paths:")
		for _, p := range excludedPaths {
			LogMessage(LOG_INFO, " |", p)
		}
	}

	if len(config.Input.Path) > 0 {
		LogMessage(LOG_VERBOSE, "(INIT)", "Searching for the following paths patterns in your drives:")
		for _, p := range config.Input.Path {
			LogMessage(LOG_INFO, " |", p)
		}
	}

	if runtime.GOOS != "windows" {
		sort.Slice(basePaths, func(i, j int) bool {
			return len(basePaths[i]) > len(basePaths[j])
		})
	}

	return basePaths, excludedPaths
}

// InitTriageScan convert fastfinder scan routine to triage infinite scan
func InitTriageScan(config Configuration, rules *yara.Rules, baseDrives []string, excludedPaths []string) {
	// init filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// enumerate drive paths and add paths to watch
	var pathsEnumeration []string
	for _, drive := range baseDrives {
		pathsEnumeration = append(pathsEnumeration, *ListDirectoryRecursively(drive, excludedPaths)...)
	}

	var pathRegexPatterns []*regexp2.Regexp
	for _, pattern := range config.Input.Path {
		re := regexp2.MustCompile(pattern, regexp2.IgnoreCase)
		pathRegexPatterns = append(pathRegexPatterns, re)
	}

	for _, p := range *PathsFinder(&pathsEnumeration, pathRegexPatterns) {
		LogMessage(LOG_INFO, "(INFO)", "Add to watchlist:", p)
		err = watcher.Add(p)
		if err != nil {
			LogFatal(fmt.Sprintf("watcher error %v", err))
		}
	}

	LogMessage(LOG_INFO, "(INFO)", "==== TRIAGE SCAN INITIALIZATION FINISHED - READY TO SCAN ===")
	time.Sleep(3 * time.Second)

	// scan routine
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				LogMessage(LOG_VERBOSE, "(INFO)", "Scanning file:", event.Name)
				time.Sleep(500 * time.Millisecond)
				m := FindInFilesContent(&[]string{event.Name}, config.Input.Content.Grep, rules, config.Input.Content.Checksum, true, config.AdvancedParameters.MaxScanFilesize, config.AdvancedParameters.CleanMemoryIfFileGreaterThanSize)
				if len(*m) > 0 && config.Output.CopyMatchingFiles {
					if config.Output.CopyMatchingFiles {
						LogMessage(LOG_VERBOSE, "(INFO)", "Copying file:", event.Name)
						FileCopy(event.Name, config.Output.FilesCopyPath, config.Output.Base64Files)
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			LogMessage(LOG_ERROR, "Watcher error:", err)
		}
	}
}
