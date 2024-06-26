{
  description = "Nix flake for ToxStatus";
  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      toxStatusVersion = "2.0.0-dev1";
    in {
      packages = flake-utils.lib.flattenTree rec {
        default = toxstatus;
        toxstatus = with pkgs; buildGoModule rec {
          pname = "toxstatus";
          version = toxStatusVersion;
          src = ./.;

          subPackages = [ "cmd/toxstatus" ];
          vendorHash = "sha256-kD00o5RpT+qq9QJAUR5NLgwMQn6IofkdpWk2mbfBa2g=";

          ldflags = let
            pkgPath = "github.com/Tox/ToxStatus/internal/version";
          in [
            "-X ${pkgPath}.Number=${version}"
            "-X ${pkgPath}.Revision=${self.shortRev or "dirty"}"
            "-X ${pkgPath}.RevisionTime=${toString self.lastModified}"
          ];

          doCheck = false;
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
