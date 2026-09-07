class X2socks < Formula
  desc "Local SOCKS5 manager based on Xray Core"
  homepage "https://github.com/xjetry/xray2socks"
  version "0.2.0"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/xjetry/xray2socks/releases/download/v0.2.0/x2socks-darwin-amd64"
      sha256 "3f4d69fb40064adf57afb308fafef01ea098cd1c396602147e0ed2b9c98a763a"
    end
    if Hardware::CPU.arm?
      url "https://github.com/xjetry/xray2socks/releases/download/v0.2.0/x2socks-darwin-arm64"
      sha256 "9b0115b47038a9557b20f8315d2fd85583f4ff4675600107ef08d41159024dac"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/xjetry/xray2socks/releases/download/v0.2.0/x2socks-linux-amd64"
      sha256 "aa4376244175d0da5eb4a0224a5cf3abb13218cefae7872ef0b46c0fbbc889a3"
    end
    if Hardware::CPU.arm?
      url "https://github.com/xjetry/xray2socks/releases/download/v0.2.0/x2socks-linux-arm64"
      sha256 "ae5b2b98f79cbc8b072ffa44789dbc7715384a448c357116865c6caf36a57edc"
    end
  end

  def install
    binary = Dir["x2socks-*"].first
    chmod 0755, binary
    bin.install binary => "x2socks"
  end

  test do
    assert_match "Usage", shell_output("#{bin}/x2socks --help 2>&1")
  end
end
