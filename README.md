# SyncLink

**Inspired by Scoop's `persist` feature, `synclink` helps you manage configuration files or any other folders/files by moving them to a central synchronized directory (like Dropbox, Google Drive, OneDrive, etc.) and creating symbolic links (symlinks) at the original location.**

This is particularly useful for applications that store configuration data in non-standard locations (e.g., `%APPDATA%\Local`), which might not be covered by tools like Scoop's built-in persistence mechanism. `synclink` automates the process of moving these files and linking them back, facilitating easy backup and synchronization across multiple machines.

## Problem Example

Tools like [uv](https://github.com/astral-sh/uv) might be installed via Scoop, but store their configurations in locations like `%APPDATA%/Local/uv`. Standard Scoop persistence doesn't manage these external locations. `synclink` allows you to move the `uv` configuration folder to your sync directory and create a symlink, ensuring your settings are backed up and synchronized without manual intervention.

## Features

*   **Move & Link:** Moves target files or folders to a designated sync directory and creates a symbolic link at the original path.
*   **Centralized Management:** Keeps track of all created links.
*   **Shortcut Creation:** Optionally creates Start Menu shortcuts for linked items.
*   **Robust File Handling:** Includes progress indicators for cross-disk move operations (planned/implemented). Handles both files and folders. Files are stored in a dedicated `files` subdirectory within the sync path.
*   **Link Maintenance:** Commands to list, remove (`unlink`), and recreate (`relink`) managed links and shortcuts.
*   **Configuration:** Manage settings like the default sync path via a `config` command, similar to `git config`.

## Installation

*(Provide instructions here once available. Examples:)*

*   **Download:** Grab the latest executable from the [Releases](https://github.com/your-username/synclink/releases) page.
*   **Scoop (Recommended):**
    ```powershell
    scoop bucket add scoop-tools https://github.com/xuwenbolan/scoop-tools-bucket
    scoop install synclink
    ```
*   **Build from Source:** *(Add build instructions if applicable)*

## Usage

`synclink` operates through several commands:

---

### `synclink link <target_path>`

Moves a target file or folder to the sync directory and creates a symbolic link at the original location.

```bash
synclink link <target_path> [-n <link_name>] [-s <sync_path>] [--shortcut] [--unlink]
```

**Arguments & Options:**

*   `<target_path>`: (Required) The path to the file or folder you want to manage. After successful execution, this path will become a symbolic link pointing to the item in the sync directory.
*   `-n, --name <link_name>`: (Optional) The name used to identify this link within `synclink`. Defaults to the base name of `<target_path>`.
*   `-s, --sync-path <sync_path>`: (Optional) Specifies the parent directory *within* your main sync root where this specific item should be stored. Defaults to `DefaultSyncPath` (root of the sync directory). Folders are stored directly under this path, while files are stored in a `files` subfolder (e.g., `{sync_path}\files\{link_name}`).
*   `--shortcut`: (Optional) If present, creates a shortcut for the *original* `<target_path>` in the Windows Start Menu in addition to creating the symlink.
*   `--unlink`: (Optional) If present, `synclink` will *not* move the file or create a symlink. This flag is primarily used in conjunction with `--shortcut` to only create a Start Menu shortcut without managing the file/folder itself via symlinking.

**Example:**

```bash
# Move C:\Users\You\AppData\Local\uv and link it, storing it in the default sync path
synclink link C:\Users\You\AppData\Local\uv

# Move C:\MyTool\config.json, call the link 'mytool-config', store in 'configs' sub-sync dir
synclink link C:\MyTool\config.json -n mytool-config -s %USERPROFILE%\OneDrive\SyncedConfigs

# Only create a Start Menu shortcut for an executable, don't move or link it
synclink link C:\ProgramFiles\MyApp\App.exe --shortcut --unlink -n MyAppLauncher
```

---

### `synclink unlink <link_name>`

Removes a managed link.

```bash
synclink unlink <link_name>
```

**Arguments:**

*   `<link_name>`: The name of the link (as specified with `-n` during `link`, or the default name) to remove.
    *   If the link is a **symbolic link**: The file/folder from the sync directory is moved back to the original location, and the symlink is deleted.
    *   If the link was created **only as a shortcut** (using `--shortcut --unlink`): Only the Start Menu shortcut is deleted.
    *   If `<link_name>` is `*`: Attempts to unlink *all* managed items. Use with caution.

**Example:**

```bash
# Unlink the item named 'uv' (moves files back, removes symlink)
synclink unlink uv

# Remove the shortcut named 'MyAppLauncher'
synclink unlink MyAppLauncher

# Unlink all managed items
synclink unlink *
```

---

### `synclink relink <link_name>`

Checks and potentially recreates a managed link or shortcut if it's missing or broken.

```bash
synclink relink <link_name>
```

**Arguments:**

*   `<link_name>`: The name of the link to check.
    *   For **symbolic links**: Verifies if the symlink exists at the original path and points correctly. If not, it attempts to recreate the symlink (assuming the target still exists in the sync directory). It does *not* move files back.
    *   For **shortcuts**: Verifies if the shortcut exists in the Start Menu. If not, it attempts to recreate it.
    *   If `<link_name>` is `*`: Checks and potentially recreates *all* managed items.

**Example:**

```bash
# Check and potentially recreate the link for 'uv'
synclink relink uv

# Check and potentially recreate all links/shortcuts
synclink relink *
```

---

### `synclink list`

Displays a list of all items currently managed by `synclink`.

```bash
synclink list
```

**Output:** Shows the `link_name`, original path, sync path target, and type (symlink/shortcut).

---

### `synclink config <get|set> <attribute> [new_value]`

Gets or sets configuration values for `synclink`.

```bash
synclink config get <attribute>
synclink config set <attribute> <new_value>
```

**Arguments:**

*   `get|set`: Action to perform.
*   `<attribute>`: The configuration key (e.g., `DefaultSyncPath`).
*   `[new_value]`: Required only for `set`. The new value for the attribute.

**Example:**

```bash
# Set the default directory where linked items will be stored
synclink config set DefaultSyncPath C:\Users\You\Dropbox\SyncedStuff

# Get the current default sync path
synclink config get DefaultSyncPath
```

---

## Configuration File

`synclink` stores its configuration (including the list of managed links and settings) in a file located at: *(Specify location, e.g., `%APPDATA%\synclink\config.json` or `%USERPROFILE%\.config\synclink\config.json`)*

Key settings include:
*   `DefaultSyncPath`: The root directory used for storing linked items if `-s` is not specified during `link`.

---

## Future Plans / Roadmap

*   **Pointer Command (`synclink pointer`):** *(This feature is not yet implemented)* The idea is to create small launcher executables (`pointer.exe`). These launchers would contain embedded information (like a regex path) read at runtime to find and launch the actual target executable, potentially allowing for more complex or relative path scenarios where simple shortcuts or symlinks are insufficient.

## Contributing

Contributions are welcome! Please feel free to submit Pull Requests or open Issues on the [GitHub repository](https://github.com/your-username/synclink).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
