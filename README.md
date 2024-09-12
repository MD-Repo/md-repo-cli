# md-repo-cli
A command-line interface for MD-Repo service



## Download pre-built binary
Download a binary for your target system architecture.

For Mac OS on Intel CPU (darwin-amd64):
```bash
CLI_VER=$(curl -L -s https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt); \
curl -L -s https://github.com/MD-Repo/md-repo-cli/releases/download/${CLI_VER}/mdrepo-${CLI_VER}-darwin-amd64.tar.gz | tar zxvf -
```

For Mac OS on M1/M2/M3 CPU (darwin-arm64):
```bash
CLI_VER=$(curl -L -s https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt); \
curl -L -s https://github.com/MD-Repo/md-repo-cli/releases/download/${CLI_VER}/mdrepo-${CLI_VER}-darwin-arm64.tar.gz | tar zxvf -
```

For Linux on Intel CPU (linux-amd64):
```bash
CLI_VER=$(curl -L -s https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt); \
curl -L -s https://github.com/MD-Repo/md-repo-cli/releases/download/${CLI_VER}/mdrepo-${CLI_VER}-linux-amd64.tar.gz | tar zxvf -
```

For Linux on ARM CPU (linux-arm64):
```bash
CLI_VER=$(curl -L -s https://raw.githubusercontent.com/MD-Repo/md-repo-cli/main/VERSION.txt); \
curl -L -s https://github.com/MD-Repo/md-repo-cli/releases/download/${CLI_VER}/mdrepo-${CLI_VER}-linux-arm64.tar.gz | tar zxvf -
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