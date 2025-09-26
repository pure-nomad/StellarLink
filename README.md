# StellarLink

A stable, just-works reverse shell manager designed for streamlined operations.

## Features

- Manage multiple reverse shells without killing the connection  
- Interactive sessions via CLI or **Discord** via [CordKit](https://github.com/pure-nomad/cordkit)  
- Real-time Discord notifications  
- Supports both **Windows** and **UNIX** shells  

### Reverse Shell Management  
- Full support for both **Windows** and **UNIX-based** reverse shells  
- Effortlessly enter, exit, or close sessions without disrupting your access  

### CordKit Integration  
- Execute commands remotely from your phone using Discord slash commands  
- Instantly list and select sessions right from the Discord interface  
- Remotely clean up dead sessions or channels — no terminal needed  

### Notifications  
- Get notified in real-time the moment a new connection is established  
- Auto-generated transcripts to keep a record of every session  
- Organized channel structure keeps live and dead sessions cleanly separated  

### Bot Commands
- /list: List active connections
- /select <id>: Select a connection to interact with
- /cmd <command>: Run a command on the selected session

## Setup

1. Clone the repository:
```sh
git clone https://github.com/pure-nomad/stellarlink.git && cd stellarlink
```

2. Create the config, there is an example one provided in the repository, refer to [CordKit](https://github.com/pure-nomad/cordkit) documentation for better understanding.

3. Build stellarlink:
```sh
go build stellarlink.go
```

## Usage

Run the StellarLink server:
```sh
./stellarlink -c ./config.json
```

## Ethical Usage

StellarLink is developed strictly for ethical and educational purposes. Unauthorized use of this tool against systems or networks without explicit consent is illegal and unethical. The creator of this project assumes no liability for misuse.
