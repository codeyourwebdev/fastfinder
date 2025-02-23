![Fastfinder logo](./Icon.png)
# _FastFinder_ - Incident Response - Fast suspicious file finder
[![Golang](https://img.shields.io/badge/Go-1.17-blue.svg)](https://golang.org) ![Linux](https://img.shields.io/badge/Supports-Linux-green.svg) ![windows](https://img.shields.io/badge/Supports-windows-green.svg)
![build windows workflow](https://github.com/codeyourweb/fastfinder/actions/workflows/go_build_windows.yml/badge.svg) ![build windows workflow](https://github.com/codeyourweb/fastfinder/actions/workflows/go_build_linux.yml/badge.svg)

## What is this project designed for?
_FastFinder_ is a lightweight tool made for threat hunting, live forensics and triage on both Windows and Linux Platforms. It is 
focused on endpoint enumeration and suspicious file finding based on various criterias:
* file path / name
* md5 / sha1 / sha256 checksum
* simple string content match
* complex content condition(s) based on YARA

## Ready for battle!
* fastfinder has been tested in real cases in multiple CERT, CSIRT and SOC
* examples directory now include real malwares , suspect behaviors or vulnerability scan

### Installation 
Compiled release of this software are available. If you want to compile 
from sources, it could be a little bit tricky because it strongly depends of 
_go-yara_ and CGO compilation. Anyway, you'll find a detailed documentation [for windows](README.windows-compilation.md) and [for linux](README.linux-compilation.md)

### Usage 
```
  ___       __  ___  ___         __   ___  __
 |__   /\  /__`  |  |__  | |\ | |  \ |__  |__)
 |    /~~\ .__/  |  |    | | \| |__/ |___ |  \

  2021-2022 | Jean-Pierre GARNIER | @codeyourweb
  https://github.com/codeyourweb/fastfinder  

usage: fastfinder [-h|--help] [-c|--configuration "<value>"] [-b|--build
                  "<value>"] [-o|--output "<value>"] [-n|--no-window]
                  [-u|--no-userinterface] [-v|--verbosity <integer>]
                  [-t|--triage]

                  Incident Response - Fast suspicious file finder

Arguments:

  -h  --help              Print help information
  -c  --configuration     Fastfind configuration file. Default:
  -b  --build             Output a standalone package with configuration and
                          rules in a single binary
  -o  --output            Save fastfinder logs in the specified file
  -n  --no-window         Hide fastfinder window
  -u  --no-userinterface  Hide advanced user interface
  -v  --verbosity         File log verbosity
                                 | 4: Only alert
                                 | 3: Alert and errors
                                 | 2: Alerts,errors and I/O operations
                                 | 1: Full verbosity)
                                . Default: 3
  -t  --triage            Triage mode (infinite run - scan every new file in
                          the input path directories). Default: false
``` 

Depending on where you are looking for files, _FastFinder_ could be used with admin OR simple user rights. 

### Scan and export file match according to your needs
configuration examples are available [there](./examples)
``` 
input:
    path: [] # match file path AND / OR file name based on simple string 
    content:
        grep: [] # match literal string value inside file content
        yara: [] # use yara rule and specify rules path(s) for more complex pattern search (wildcards / regex / conditions) 
        checksum: [] # parse for md5/sha1/sha256 in file content 
options:
    contentMatchDependsOnPathMatch: true # if true, paths are a pre-filter for content searchs. If false, paths and content both generate matchs
    findInHardDrives: true	# enumerate hard drive content
    findInRemovableDrives: true # enumerate removable drive content 
    findInNetworkDrives: true # enumerate network drive content
    findInCDRomDrives: true # enumerate physical CD-ROM and mounted iso / vhd...
output:
    copyMatchingFiles: true # create a copy of every matching file
    base64Files: true # base64 matched content before copy
    filesCopyPath: '' # empty value will copy matched files in the fastfinder.exe folder
advancedparameters:
    yaraRC4Key: ''    # yara rules can be (un)/ciphered using the specified RC4 key
    maxScanFilesize: 2048 #  ignore files up to maxScanFileSize Mb (default: 2048)               
    cleanMemoryIfFileGreaterThanSize: 512 # clean fastfinder internal memory after heavy file scan (default: 512Mb) 
``` 
### Note for input path:
* '?' for simple char  and '\\*' (eg. powershe??.exe)  for multiple chars (eg. \\*.exe) wildcards are available for simple string
* environment variables are also available (eg. %TEMP%\\\*.exe)
* regular expression are allowed , they should be enclosed by /<regex>/
* input path are always case INSENSITIVE
* input content grep strings are always case SENSITIVE
* backslashes haven't to be escaped on simple string pattern
For more informations, take a look at the [examples](./examples)

## About this project
I initially created this project to automate fast system oriented IOC detection on a wide computer network. 
It fulfills the needs I have today. Nevertheless if you have complementary ideas, do not hesitate 
to ask for, I will see to implement them if they can be useful for everyone.
On the other hand, pull request will be studied carefully.

## Future releases
I don't plan to add any additional features right now. The next release will be focused on:
* Unit testing / Code testing coverage / CI
* Build more examples based on live malwares tradecraft and threat actor campaigns
