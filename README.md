# cho

choice!

![cho](https://raw.githubusercontent.com/mattn/cho/master/cho.gif)

## Why cho?

Why not `choice`? Because Windows already have `choice` command.

Why not `choic`? Too long

Why not `choi`? Still long

Then, `cho`? Sound good to me

## Usage

Just like [peco](https://github.com/peco/peco)

## Installation

```
$ go get github.com/mattn/cho
```

## Usecase

### Linux
```
FOO=`ls | cho`
```

### Windows

```
for /f "delims=;" %%i in ('ls ^| cho') do set FOO=%%i
```

## Keys

|Key   |Behavior            |
|------|--------------------|
|CTRL-N|Next                |
|CTRL-P|Previous            |
|Enter |Decide              |
|CTLR-A|Left side on prompt |
|CTLR-E|Right side on prompt|
|CTRL-V|Select in multi mode|
|CTRL-U|Clear prompt        |
|ESC   |Cancel              |

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
