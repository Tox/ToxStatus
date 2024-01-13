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
          vendorHash = "sha256-cE8L7vuf3msMplL6BfQ4gt1rELMIFswDu9mfSXJ9VAs=";
        };
      };
      devShell = with pkgs; mkShell {
        hardeningDisable = [ "fortify" ];
        buildInputs = [
          go
          graphviz # for pprof
          sqlite
        ];
      };
    }
  );
}
