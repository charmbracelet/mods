{ pkgs }:

pkgs.buildGo118Module {
  name = "mods";
  src = ./.;
  vendorSha256 = "sha256-GNGX8dyTtzRSUznEV/do1H7GEf6nYf0w+CLCZfkktfg=";
}
