{
  description = "Nix flake for ToxStatus";
  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      packages = flake-utils.lib.flattenTree rec {
        default = toxstatus;
        toxstatus = with pkgs; buildGoModule rec {
          name = "toxstatus";
          src = ./.;

          subPackages = [ "cmd/toxstatus" ];
          vendorHash = "sha256-7Z/ShmVKlx64xH0JiDv97y/KbEDw8IOUJJ2h02h+zRE=";
        };
      };
      devShell = with pkgs; mkShell {
        buildInputs = [
          go
        ];
      };
    }
  );
}
