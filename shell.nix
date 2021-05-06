let _pkgs = import <nixpkgs> { };
in
{ pkgs ?
  import
    (_pkgs.fetchFromGitHub {
      owner = "NixOS";
      repo = "nixpkgs-channels";
      #branch@date: nixpkgs-unstable@2020-12-17
      rev = "00941cd747e9bc1c3326d1362dbc7e9cfe18cf53";
      sha256 = "12mjfar2ir561jxa1xvw6b1whbqs1rq59byc87icql399zal5z4a";
    }) { }
}:

with pkgs;

mkShell {
  GOROOT = "";
  buildInputs = [
    git
    gnumake
    gnused
    go
    gotools
    jq
    shfmt
    shellcheck
    libseccomp
    rootlesskit
    runc
  ];
}
