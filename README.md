# gxuitter

twitter client using [GXUI](https://github.com/google/gxui)

## Installation

```
$ go get github.com/mattn/gxuitter
```

## Configurations

gxuitter store configurations in a file:

#### UNIX

`~/.config/gxuitter/settings.json`

#### Windows

`%APPDATA%\Roadming\gxuitter\settings.json`

If you want to change font,

```json
{
  "AccessSecret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "AccessToken": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "ClientSecret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "ClientToken": "XXXXXXXXXXXXXXXXXXXXXX",
  "FontFile": "/path/to/truetype-font-file.ttf",
  "FontSize": 12
}
```

## TODO

* Input Method doesn't work for GXUI ()
  * Issue [#60](https://github.com/google/gxui/issues/60) on GXUI
  * Issue [#473](https://github.com/glfw/glfw/pull/473) on glfw
* Interval timer for updating statuses
* Retweeeeeeeeeeeet
* Favorite
* Inline image in the tweet

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
