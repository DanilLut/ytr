# ytr - YouTube Randomizer

ytr (YouTube Randomizer) is a simple terminal-based application for managing a collection of YouTube videos. It allows users to add videos, list them, delete entries, and open a random video in the web browser.

## Features

- Add YouTube videos to the saved list
- Play a random video from the saved list
- Manage videos in a TUI (add, search, delete, play random, play selected)

## Installation

```sh
go install github.com/DanilLut/ytr/v2@latest
pipx install yt-dlp
```

### Install wslu, if using WSL:
```sh
sudo apt install wslu
```

## Usage

Run the program with different options:

### Play a random video from the saved list (removes video from the list automatically)
```sh
ytr
```

### Add a YouTube video
```sh
ytr <YouTube URL>
```

### Launch TUI mode
```sh
ytr -t
```

### Show help
```sh
ytr -h
```

## Configuration & Storage
The application stores video entries in a JSON database located at:

- **Linux/macOS/WSL**: `~/.config/ytr/data.json`
- **Windows**: `%APPDATA%\ytr\data.json`

## Dependencies
This project uses the following Go libraries:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styling
- [Color](https://github.com/fatih/color) for colored output

## License
This project is licensed under the MIT License. See `LICENSE` for details.

## Contributions
Pull requests and feature suggestions are welcome! Feel free to open an issue or contribute to the project.

