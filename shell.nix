with import <nixpkgs> {};
mkShell {
  buildInputs = [
    go
    gnumake
  ];
}
