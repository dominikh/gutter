// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

type EmbeddingPermissions int

const (
	InstallableEmbedding       EmbeddingPermissions = 1
	RestrictedLicenseEmbedding EmbeddingPermissions = 2
	PreviewAndPrintEmbedding   EmbeddingPermissions = 3
	EditableEmbedding          EmbeddingPermissions = 4
)

type EmbeddingLicense struct {
	Permissions         EmbeddingPermissions
	NoSubsetting        bool
	BitmapEmbeddingOnly bool
}

func (tbl *OS2Table) EmbeddingLicense() EmbeddingLicense {
	var usage EmbeddingPermissions
	switch tbl.FsType & 0xF {
	case licenseInstallableEmbedding:
		usage = 1
	case licenseRestrictedLicenseEmbedding:
		usage = 2
	case licensePreviewAndPrintEmbedding:
		usage = 3
	case licenseEditableEmbedding:
		usage = 4
	}

	return EmbeddingLicense{
		Permissions:         EmbeddingPermissions(usage),
		NoSubsetting:        tbl.FsType&0x0100 != 0,
		BitmapEmbeddingOnly: tbl.FsType&0x0200 != 0,
	}
}
