// Command translate 是語言轉換工具的 CLI 前端。
package main

import (
	gosdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/trans/cli"
	appconfig "github.com/bizshuk/trans/config"
)

func main() {
	appconfig.Default(gosdkconfig.WithAppName("trans"))
	cli.Execute()
}
