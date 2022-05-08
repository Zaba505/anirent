/*
Copyright © 2022 Zaba505 <cakub6@gmx.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logLevel zapcore.Level

func (l logLevel) String() string {
	return (zapcore.Level)(l).String()
}

func (l *logLevel) Set(s string) error {
	return (*zapcore.Level)(l).Set(s)
}

func (l logLevel) Type() string {
	return "Level"
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "anirent [ANIME]",
	Short: "",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var lvl zapcore.Level
		lvlStr := cmd.Flags().Lookup("log-level").Value.String()
		err := lvl.UnmarshalText([]byte(lvlStr))
		if err != nil {
			panic(err)
		}

		l, err := zap.NewDevelopment(zap.IncreaseLevel(lvl))
		if err != nil {
			panic(err)
		}

		zap.ReplaceGlobals(l)
	},
}

func init() {
	// Persistent flags
	lvl := logLevel(zapcore.WarnLevel)
	rootCmd.PersistentFlags().VarP(&lvl, "log-level", "l", "Specify log level")
}
