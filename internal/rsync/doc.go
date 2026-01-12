// Package rsync implements a native Go rsync client for Gentoo repository synchronization.
//
// This is a minimal rsync implementation focused on receiving files from rsync:// mirrors.
// It supports protocol version 27 with zlib compression.
//
// # Features
//
//   - Receiver (client) mode only
//   - Full file transfers (no delta/rsync algorithm)
//   - Zlib compression (-z flag) - unlike gokrazy/rsync which doesn't support compression
//   - Delete mode (--delete) - removes files not present on server
//   - Protocol version 27 (compatible with Gentoo mirrors)
//   - File list compression (shared name prefixes)
//   - Multiplexed I/O (data + error messages)
//
// # Usage
//
// This implementation is designed specifically for Gentoo Portage repository sync:
//
//	client := rsync.NewClient()
//	client.Compress = true  // Enable compression
//	client.Delete = true    // Enable --delete
//	err := client.Sync(ctx, "rsync://rsync.gentoo.org/gentoo-portage/", "/var/db/repos/gentoo")
//
// # Protocol Reference
//
// The implementation is based on the original rsync C source code, particularly:
//   - token.c: compression/decompression token protocol
//   - flist.c: file list transmission format
//   - io.c: multiplexed I/O with message tags
//   - clientserver.c: rsync:// protocol handshake
//
// Key protocol details:
//   - Raw deflate compression (no zlib header), -15 window bits
//   - Z_SYNC_FLUSH markers (00 00 FF FF) between blocks
//   - 14-bit length encoding for compressed chunks (max 16383 bytes)
//   - XMIT_* flags for incremental file list encoding
//
// # Future Plans
//
// This package will be extracted into a standalone library github.com/grpmsoft/go-rsync
// with full client+server support for general-purpose rsync operations.
package rsync
