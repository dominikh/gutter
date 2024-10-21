// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type sizeDelimiter [0]byte
type versionDelimiter [0]byte

// stubs
func ParseDeviceTable(buf []byte, out *DeviceTable) int                 { return 0 }
func ParseVariationIndexTable(buf []byte, out *VariationIndexTable) int { return 0 }
