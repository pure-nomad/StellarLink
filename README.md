# StellarLink

A stable, just-works reverse shell manager designed for streamlined operations.

## Features
- Efficient reverse shell management
- Integration for Windows & *nix reverse shells.
- Discord Webhook Integration for notifications

## Setup

1. Clone the repository:
```sh
git clone https://github.com/pure-nomad/stellarlink.git && cd stellarlink
```

2. Replace the Discord webhook constant (`discordWebhookURL`) in the source code with your discord webhook URL.

3. Modify the server's bind address in the source code to your desired IP and port. Default is `localhost:8080`.

4. When steps 2 and 3 are done, build stellarlink:
```sh
go build stellarlink.go
```

## Usage

Create your config, this project uses [Cordkit]("https://github.com/pure-nomad/cordkit" "Cordkit") so refer to that documentation for better understanding.

Run the StellarLink server:
```sh
./stellarlink -c ./config.json
```

## Ethical Usage

StellarLink is developed strictly for ethical and educational purposes. Unauthorized use of this tool against systems or networks without explicit consent is illegal and unethical. The creator of this project assumes no liability for misuse.

