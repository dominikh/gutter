# SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
#
# SPDX-License-Identifier: MIT

{
  inputs = { nixpkgs.url = "github:NixOS/nixpkgs"; };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShells.${system}.default = with pkgs;
        mkShell {
          packages = [
            vulkan-headers
            libxkbcommon
            wayland
            xorg.libX11
            xorg.libXcursor
            xorg.libXfixes
            libGL
            harfbuzz
            pkg-config
          ];
          LD_LIBRARY_PATH = "${vulkan-loader}/lib";
        };
    };
}
