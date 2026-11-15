class Mihosh < Formula
  desc "Terminal UI management tool for mihomo"
  homepage "https://github.com/AimAI-Labs/mihosh"
  version "2.0.0-rc.3"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.3/mihosh-darwin-arm64.tar.gz"
      sha256 "06e23cb3b351fefd6577a99b8ffce4b8b63b6d9573377848310f518d0c71b0e3"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.3/mihosh-darwin-amd64.tar.gz"
      sha256 "47e12baf99e5acfceff3c9b451428c4ad2e0676b30b3882eb77bfbd10c4349bf"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.3/mihosh-linux-arm64.tar.gz"
      sha256 "376a9723da014ecd3d6dd6212aef16f4bb5889988e6f38e6d3a4793b0eb5f17f"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.3/mihosh-linux-amd64.tar.gz"
      sha256 "1a32d1c851189c4f94bdab81291b95b3dd6a683701798f40b60005677f247844"
    end
  end

  def install
    bin.install "mihosh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mihosh version")
  end
end
