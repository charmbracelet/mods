{
  description = "AI for the command line";

  inputs = {
    nixpkgs.url = github:nixos/nixpkgs/nixos-22.05;
    flake-utils.url = github:numtide/flake-utils;
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = import nixpkgs { inherit system; }; in
      rec {
        packages.default = import ./default.nix { inherit pkgs; };
      }) // {
        overlays.default = final: prev: rec {
          buildGoModule = final.buildGo118Module;
          mods = import ./default.nix { pkgs = final; };
        };
      };
}
