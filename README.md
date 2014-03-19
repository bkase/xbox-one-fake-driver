Forked from kylelemons/xbox

# XboxONE "joystick driver" for OSX

Hacked together "joystick driver" for XboxOne controllers on OSX -- converts Xbox One controller input into keyboard keystrokes. I took [Kyle Lemmons' Xbox One data packet parsing code](https://github.com/kylelemons/xbox) and modified it to dump key commands to my [key-injector server](https://github.com/bkase/key-injector).

Plays [Risk of Rain](http://riskofraingame.com/) on an emulator well!

## How it works

I basically combined Kyle Lemons' Xbox 360 code and Xbox One code and discretized the joystick input (so it can be translated to something like W,A,S,D or the arrow keys). Then the "driver" dumps key commands over Unix Domain sockets to the [key-injector server](https://github.com/bkase/key-injector) which plays them to the OS.
## How to use

### Install

First install the [key-injector server](https://github.com/bkase/key-injector)

Then run:

```bash
go get github.com/bkase/xbox
```

## To Run

Start the key-injector server:

```bash
python key-injector.py
```

Then **plug in** your Xbox One controller through the micro-usb port.

Then run:

```bash
cd $GOPATH/src/github.com/bkase/xbox
go run xbox.go
```

You'll need to re-plug the controller before you run the binary, (bug inherited from kylelemons/xbox ).

