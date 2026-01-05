# EMO Proxy server

This part of the knowledge bank is a Proxy server to analyze the traffic that goes between the EMO robot and the living.ai servers.

## Start the proxy

`go run .`

### Alternatively start with air

`air run .`

## Docker setup

You can find a simplified setup using Docker Compose by SebbeJohansson here: https://github.com/emo-libre/emo-proxy-docker
It uses nginx, dnsmasq, and mitmproxy to be able to pass through the api requests to the EMO Proxy.

## Experimental

### ChatGPT Speak server

One of the most common responses from EMO is to respond using ChatGPT.
An experimental feature is to use a local Speak server to generate EMO's chatgpt speak responses instead of using the living.ai servers.
You can find a proof of concept server here: https://github.com/SebbeJohansson/EmoChatGptSpeakPOC

Connect the server by adding the following line to your emoProxy.conf file:
`"chatGptSpeakServer": "http://localhost:8085"`
