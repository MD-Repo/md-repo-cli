import requests
import os
import re

# GitHub repository details
REPO = "MD-Repo/md-repo-cli"
API_URL = f"https://api.github.com/repos/{REPO}/releases/latest"

def get_latest_release_data():
    headers = {
        'Authorization': f'token {os.getenv("GITHUB_TOKEN")}',
        'Accept': 'application/vnd.github.v3+json',
    }
    response = requests.get(API_URL, headers=headers)
    response.raise_for_status()
    return response.json()

def extract_os_arch_from_filename(filename):
    # Assuming filenames follow the pattern: <name>-<version>-<os>-<arch>.<ext>
    pattern = r'[-]([a-zA-Z]+)[-]([a-zA-Z0-9_]+)[.]'
    
    match = re.search(pattern, filename)
    
    if match:
        os = match.group(1)
        arch = match.group(2)

        if os == "darwin":
            os = "MacOS"
        elif os == "linux":
            os = "Linux"
        elif os == "windows":
            os = "Windows"

        if arch == "amd64":
            arch = "Intel/AMD 64-bit"
        elif arch == "386":
            arch = "Intel/AMD 32-bit"
        elif arch == "arm64":
            arch = "ARM 64-bit"
        elif arch == "arm":
            arch = "ARM 32-bit"

        return os, arch
    else:
        return None, None

def generate_markdown_table(release):
    table = "### Release Assets\n"
    table += "| OS | Architecture | Link |\n"
    table += "|---------|----------|-------------|\n"
    
    for asset in release["assets"]:
        if not asset["name"].endswith(".md5"):
            # parse
            os, arch = extract_os_arch_from_filename(asset["name"])
            download_url = asset["browser_download_url"]
            table += f"| {os}  | {arch}  | [Download]({download_url}) |\n"
    
    return table

def update_release_body(release_id, new_body):
    headers = {
        'Authorization': f'token {os.getenv("GITHUB_TOKEN")}',
        'Accept': 'application/vnd.github.v3+json',
    }
    update_url = f"https://api.github.com/repos/{REPO}/releases/{release_id}"
    data = {
        "body": new_body
    }
    
    response = requests.patch(update_url, headers=headers, json=data)
    response.raise_for_status()

"""
def update_readme(table):
    with open("README.md", "r") as file:
        lines = file.readlines()

    with open("README.md", "w") as file:
        inside_table = False
        for line in lines:
            if line.startswith("| Version"):
                inside_table = True
                file.write(table)
                continue
            if inside_table and line.startswith("|"):
                continue
            file.write(line)
"""

if __name__ == "__main__":
    latest_release = get_latest_release_data()
    release_id = latest_release["id"]
    current_body = latest_release.get("body", "")

    markdown_table = generate_markdown_table(latest_release)
    print(markdown_table)

    if "### Release Assets\n" not in current_body:
        updated_body = current_body + "\n\n" + markdown_table
        update_release_body(release_id, updated_body)
        print("updated release body")
