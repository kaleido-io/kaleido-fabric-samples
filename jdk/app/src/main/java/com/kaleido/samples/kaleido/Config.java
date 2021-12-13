package com.kaleido.samples.kaleido;

import java.nio.file.Paths;

public class Config {
  private String rootdir;

  public Config() {
    this.rootdir = Paths.get(System.getProperty("user.home"), "kaleido-fabric-java").toString();
  }

  public String getKaleidoUrl() {
    String url = System.getenv("KALEIDO_URL");
    if (url == null) {
      url = "https://console.kaleido.io/api/v1";
    }
    return url;
  }

  public String getAPIKey() {
    String key = System.getenv("APIKEY");
    if (key == null) {
      System.out.println("APIKEY environment variable must be set");
      System.exit(1);
    }
    return key;
  }

  public String getUsername() {
    String username = System.getenv("USER_ID");
    if (username == null) {
      username = "user01";
    }
    return username;
  }

  public String getRootDir() {
    return this.rootdir;
  }

  public String getContractName() {
    String contract = System.getenv("CCNAME");
    if (contract == null) {
      contract = "asset_transfer";
    }
    return contract;
  }
}
