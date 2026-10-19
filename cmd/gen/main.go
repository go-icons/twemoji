// Copyright (c) 2026 the go-icons authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command gen regenerates twemoji.bin and shortcodes.txt from upstream:
//
//	git clone --depth 1 --branch v17.0.3 https://github.com/jdecked/twemoji
//	git clone --depth 1 --branch 3.0.0 https://github.com/markdown-it/markdown-it-emoji
//	go run ./cmd/gen --twemoji ./twemoji --markdown-it-emoji ./markdown-it-emoji \
//	    --emoji-sequences emoji-sequences.txt
//
// emoji-sequences.txt is Unicode's (unicode-org/unicodetools,
// unicodetools/data/emoji/17.0/).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/go-icons/twemoji/internal/gen"
)

func newRoot(stdout io.Writer) *cobra.Command {
	var twemojiDir, mdDir, sequences, out string
	cmd := &cobra.Command{
		Use:           "gen",
		Short:         "regenerate the embedded Twemoji data from upstream checkouts",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := gen.Build(
				filepath.Join(twemojiDir, "assets", "svg"),
				filepath.Join(mdDir, "lib", "data", "full.mjs"),
				sequences,
				out,
			); err != nil {
				return err
			}
			_, err := fmt.Fprintln(stdout, "wrote twemoji.bin, shortcodes.txt and textdefault.txt in", out)
			return err
		},
	}
	cmd.Flags().StringVar(&twemojiDir, "twemoji", "", "a jdecked/twemoji checkout (assets/svg inside)")
	cmd.Flags().StringVar(&mdDir, "markdown-it-emoji", "", "a markdown-it-emoji checkout (lib/data/full.mjs inside)")
	cmd.Flags().StringVar(&sequences, "emoji-sequences", "", "Unicode's emoji-sequences.txt (emoji/17.0)")
	cmd.Flags().StringVar(&out, "out", ".", "directory to write into")
	_ = cmd.MarkFlagRequired("twemoji")
	_ = cmd.MarkFlagRequired("markdown-it-emoji")
	_ = cmd.MarkFlagRequired("emoji-sequences")
	return cmd
}

func main() {
	if err := newRoot(os.Stdout).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}
