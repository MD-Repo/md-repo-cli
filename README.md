# md-repo-cli
A command-line interface for MD-Repo service



## Download pre-built binary
Be sure to download a binary for your target system architecture.

For Linux & Mac OS:
```bash
curl -fsSL https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/install_mdrepocli.sh | bash
```

For Windows on Intel CPU (windows-amd64, windows Cmd):
```bash
curl -L -s -o mdrepov.txt https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt && set /p CLI_VER=<mdrepov.txt
curl -L -s -o mdrepo.zip https://github.com/MD-Repo/md-repo-cli/releases/download/%CLI_VER%/mdrepo-%CLI_VER%-windows-amd64.zip && tar zxvf mdrepo.zip && del mdrepo.zip mdrepov.txt
```

For Windows on Intel CPU (windows-amd64, windows PowerShell):
```bash
curl -o mdrepov.txt https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt ; $env:CLI_VER = (Get-Content mdrepov.txt)
curl -o mdrepo.zip https://github.com/MD-Repo/md-repo-cli/releases/download/$env:CLI_VER/mdrepo-$env:CLI_VER-windows-amd64.zip ; tar zxvf mdrepo.zip ; del mdrepo.zip ; del mdrepov.txt
```

### Manual download binary
Please download binary file (bundled with `tar` or `zip`) at ["https://github.com/MD-Repo/md-repo-cli/releases"]("https://github.com/MD-Repo/md-repo-cli/releases").

## Command line usage

### Uploading files
Use the command:

```bash
mdrepo submit upload_directory
```

where `upload_directory` is the local parent directory of your simulation files. Enter your upload token when prompted.

If your upload is interrupted you may use the same command and token and the upload will resume.


### Downloading files
Use the command:

```bash
mdrepo get download_directory
```

where `download_directory` is the local directory where you wish to download the files. Enter your download token when prompted.

If your download is interrupted you may use the same command and token and the download will resume.
