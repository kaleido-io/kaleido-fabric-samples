package com.kaleido.samples.kaleido;

import java.io.BufferedWriter;
import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.net.URI;
import java.net.URL;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.security.cert.Certificate;
import java.security.cert.CertificateEncodingException;
import java.util.Base64;
import java.util.Iterator;
import java.util.Scanner;
import java.util.Base64.Encoder;

import javax.net.ssl.HttpsURLConnection;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonMappingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.fasterxml.jackson.dataformat.yaml.YAMLGenerator.Feature;

import org.apache.commons.codec.DecoderException;
import org.apache.commons.codec.binary.Hex;

public class BusinessNetwork {
  public class KeyAndCert {
    public String keyFile;
    public String certFile;

    public KeyAndCert(String keyFile, String certFile) {
      this.keyFile = keyFile;
      this.certFile = certFile;
    }
  }

  private JsonNode consortium;
  private JsonNode environment;
  private JsonNode myMembership;
  private JsonNode channel;
  private JsonNode myCA;

  private String kaleidoTlsCert;

  private String url;
  private String apikey;
  private String rootdir;
  private String username;
  private HttpClient client;
  private Scanner input;

  public BusinessNetwork(Scanner input) {
    Config config = new Config();
    this.url = config.getKaleidoUrl();
    this.apikey = config.getAPIKey();
    this.rootdir = config.getRootDir();
    this.username = config.getUsername();
    this.client = HttpClient.newHttpClient();
    this.input = input;
  }

  public String getCAUrl() {
    return this.myCA.get("urls").get("http").asText();
  }

  public String getCAServiceId() {
    return this.myCA.get("_id").asText();
  }

  public String getEnvironmentId() {
    return this.environment.get("_id").asText();
  }

  public String getMembershipId() {
    return this.myMembership.get("_id").asText();
  }

  public String getConnectionProfilePath() {
    return String.format("%s/kaleido-fabric-java/%s/ccp.yaml", System.getProperty("user.home"),
        this.environment.get("_id").asText());
  }

  public String getTargetChannel() {
    return this.channel.get("name").asText();
  }

  // we can trust the kaleido.io server certificate, so we
  // download it and make it available for Fabric CA Client
  public String getKaleidoTLSCert() throws IOException, CertificateEncodingException {
    if (this.kaleidoTlsCert != null) {
      return this.kaleidoTlsCert;
    }

    String urlString = this.myCA.get("urls").get("http").asText();
    URL url = new URL(urlString);
    HttpsURLConnection conn = (HttpsURLConnection) url.openConnection();
    conn.connect();
    Certificate[] certs = conn.getServerCertificates();
    String pem = "";
    Encoder encoder = Base64.getEncoder();
    for (Certificate cert : certs) {
      pem += "-----BEGIN CERTIFICATE-----\n";
      pem += this.normalizePEMString(encoder.encodeToString(cert.getEncoded()));
      pem += "-----END CERTIFICATE-----\n";
    }

    Path filename = Paths.get(this.getDatadir(), "kaleido_ca.pem");
    File dir = filename.getParent().toFile();
    if (!dir.exists()) {
      dir.mkdirs();
    }
    BufferedWriter writer = new BufferedWriter(new FileWriter(filename.toString()));
    writer.write(pem);
    writer.close();
    conn.disconnect();

    this.kaleidoTlsCert = filename.toString();
    return this.kaleidoTlsCert;
  }

  public void build() throws IOException, InterruptedException, DecoderException, CertificateEncodingException {
    this.consortium = this.getResource(
        String.format("%s/c", this.url),
        "consortia", "name", input);
    this.environment = this.getResource(
        String.format("%s/c/%s/e", this.url, this.consortium.get("_id").asText()),
        "environments", "name", input);
    this.myMembership = this.getResource(
        String.format("%s/c/%s/m", this.url, this.consortium.get("_id").asText()),
        "memberships", "org_name", input);
    this.channel = this.getResource(
        String.format("%s/c/%s/e/%s/channels", this.url, this.consortium.get("_id").asText(),
            this.environment.get("_id").asText()),
        "channels", "name", input);
    this.myCA = this.getCA();
  }

  public void writeConnectionProfile()
      throws IOException, InterruptedException, CertificateEncodingException, DecoderException {
    JsonNode nodes = this.getResources(
        String.format("%s/c/%s/e/%s/n", this.url, this.consortium.get("_id").asText(),
            this.environment.get("_id").asText()));

    ObjectMapper mapper = new ObjectMapper();
    ObjectNode network = mapper.createObjectNode();
    network.put("version", "1.1.0");
    network.put("name", "kaleido-connection-profile");

    // the "client" section
    ObjectNode client = mapper.createObjectNode();
    client.put("organization", this.myMembership.get("_id").asText());
    ObjectNode connection = mapper.createObjectNode();
    ObjectNode timeout = mapper.createObjectNode();
    ObjectNode peer = mapper.createObjectNode();
    peer.put("endorser", "3000");
    timeout.set("peer", peer);
    connection.set("timeout", timeout);
    client.set("connection", connection);
    network.set("client", client);

    // the "certificateAuthorities" section
    ObjectNode cas = mapper.createObjectNode();
    ObjectNode ca = mapper.createObjectNode();
    ObjectNode tlsCACerts = mapper.createObjectNode();
    tlsCACerts.put("path", this.getKaleidoTLSCert());
    ca.set("tlsCACerts", tlsCACerts);
    ca.put("url", this.myCA.get("urls").get("http").asText());
    cas.set(this.myCA.get("membership_id").asText(), ca);
    network.set("certificateAuthorities", cas);

    // the "orderers" and the "peers" section
    // even though the gateway service knows the entire topology,
    // we still need to specify all entries for the mutual TLS settings.
    // different from node.js and Go SDKs, the client cert and client key
    // settings are not specified under the "client" section, but as "grpcOptions"
    // under EACH node
    ObjectNode orderers = mapper.createObjectNode();
    ObjectNode peers = mapper.createObjectNode();
    KeyAndCert kc = this.getUserKeyAndCert();
    for (int i = 0; i < nodes.size(); i++) {
      JsonNode node = nodes.get(i);
      if (node.get("membership_id").asText().equals("sys--mon")) {
        continue;
      }
      ObjectNode n = mapper.createObjectNode();
      String url = null;
      if (node.get("role").asText().equals("peer")) {
        url = node.get("urls").get("peer").asText();
      } else if (node.get("role").asText().equals("orderer")) {
        url = node.get("urls").get("orderer").asText();
      }
      url = url.substring(8); // remove http:// prefix
      n.put("url", String.format("grpcs://%s:443", url));

      // the org CA PEM is returned in the "node_identity_data" field
      // which is a hex representation of the JSONified object containing
      // the "orgCA" property
      byte[] decoded = Hex.decodeHex(node.get("node_identity_data").asText());
      String str = new String(decoded, "UTF-8");
      JsonNode nodeIds = mapper.readTree(str);
      String orgCAPem = nodeIds.get("orgCA").asText();
      ObjectNode tlsCaCerts = mapper.createObjectNode();
      tlsCaCerts.put("pem", orgCAPem);
      n.set("tlsCACerts", tlsCaCerts);

      ObjectNode opts = mapper.createObjectNode();
      opts.put("clientCertFile", kc.certFile);
      opts.put("clientKeyFile", kc.keyFile);
      n.set("grpcOptions", opts);

      if (node.get("role").asText().equals("peer")) {
        peers.set(node.get("_id").asText(), n);
      } else if (node.get("role").asText().equals("orderer")) {
        orderers.set(node.get("_id").asText(), n);
      }
    }
    if (orderers.size() == 0) {
      System.out.println("No orderers found for the membership. Exiting...");
      System.exit(1);
    }
    if (peers.size() == 0) {
      System.out.println("No peers found for the membership. Exiting...");
      System.exit(1);
    }
    network.set("peers", peers);
    network.set("orderers", orderers);

    // the "organizations" section
    ObjectNode orgs = mapper.createObjectNode();
    ObjectNode org = mapper.createObjectNode();
    org.put("cryptoPath", "/tmp/msp");
    org.put("mspid", this.myMembership.get("_id").asText());
    ArrayNode orgPeers = mapper.createArrayNode();
    orgPeers.add(peers.fieldNames().next());
    org.set("peers", orgPeers);
    ArrayNode orgOrderers = mapper.createArrayNode();
    orgOrderers.add(orderers.fieldNames().next());
    org.set("orderers", orgOrderers);
    ArrayNode orgCAs = mapper.createArrayNode();
    orgCAs.add(this.myCA.get("membership_id").asText());
    org.set("certificateAuthorities", orgCAs);
    orgs.set(this.myMembership.get("_id").asText(), org);
    network.set("organizations", orgs);

    // the "channels" section
    ObjectNode channels = mapper.createObjectNode();
    ObjectNode channel = mapper.createObjectNode();
    ArrayNode channelOrderers = mapper.createArrayNode();
    Iterator<String> orderersItr = orderers.fieldNames();
    while (orderersItr.hasNext()) {
      channelOrderers.add(orderersItr.next());
    }
    channel.set("orderers", channelOrderers);
    ObjectNode channelPeers = mapper.createObjectNode();
    Iterator<String> peersItr = peers.fieldNames();
    while (peersItr.hasNext()) {
      ObjectNode channelPeer = mapper.createObjectNode();
      channelPeer.put("endorsingPeer", true);
      channelPeer.put("chaincodeQuery", true);
      channelPeer.put("ledgerQuery", true);
      channelPeer.put("eventSource", true);
      channelPeers.set(peersItr.next(), channelPeer);
    }
    channel.set("peers", channelPeers);
    channels.set(this.channel.get("name").asText(), channel);
    network.set("channels", channels);

    YAMLFactory factory = new YAMLFactory()
        .disable(Feature.WRITE_DOC_START_MARKER)
        .enable(Feature.LITERAL_BLOCK_STYLE)
        .enable(Feature.MINIMIZE_QUOTES);
    ObjectMapper yaml = new ObjectMapper(factory);
    File ccpFile = new File(this.getConnectionProfilePath());
    File dir = ccpFile.getParentFile();
    if (!dir.exists()) {
      dir.mkdirs();
    }
    System.out.println(String.format("Writing connection profile at %s", ccpFile));
    yaml.writeValue(ccpFile, network);
  }

  private JsonNode getCA() throws IOException, InterruptedException {
    JsonNode myCA = null;
    JsonNode cas = this.getResources(
        String.format("%s/c/%s/e/%s/services", this.url, this.consortium.get("_id").asText(),
            this.environment.get("_id").asText()));
    if (cas.size() > 0) {
      for (int i = 0; i < cas.size(); i++) {
        JsonNode ca = cas.get(i);
        if (ca.get("membership_id").asText().equals(this.myMembership.get("_id").asText())) {
          System.out
              .println(String.format("Found certificate authority \"%s\" for the membership", ca.get("_id").asText()));
          myCA = ca;
          break;
        }
      }
    }
    if (myCA == null) {
      System.out.println(String.format("No Certificate Authority found for selected membership, exiting..."));
      System.exit(1);
    }
    return myCA;
  }

  private JsonNode getResource(String url, String resourceName, String nameProperty, Scanner input)
      throws IOException, InterruptedException {
    JsonNode resource = null;
    JsonNode resources = this.getResources(url);
    if (resources.size() > 1) {
      System.out.println(String.format("\nFound the following %s:", resourceName));
      for (int i = 0; i < resources.size(); i++) {
        JsonNode res = resources.get(i);
        System.out
            .print(String.format("\t%s -> %s (%s)\n", i, res.get(nameProperty).asText(), res.get("_id").asText()));
        if (i == resources.size() - 1) {
          System.out.print("\t=> ");
        }
      }
      String index = input.nextLine();
      resource = resources.get(Integer.parseInt(index));
    } else if (resources.size() == 1) {
      resource = resources.get(0);
    }

    if (resource == null) {
      System.out.println(String.format("No %s found. Exiting...", resourceName));
      System.exit(1);
    }
    System.out.println(String.format("\nSelected %s \"%s\" (%s)", resourceName, resource.get(nameProperty).asText(),
        resource.get("_id").asText()));
    return resource;
  }

  private JsonNode getResources(String url) throws IOException, InterruptedException {
    HttpRequest request = HttpRequest.newBuilder()
        .uri(URI.create(url))
        .GET()
        .header("Authorization", "Bearer " + this.apikey)
        .build();
    HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());

    ObjectMapper mapper = new ObjectMapper();
    JsonNode resources = mapper.readTree(response.body());
    return resources;
  }

  private String getDatadir() {
    return Paths.get(this.rootdir, this.environment.get("_id").asText()).toString();
  }

  // break up a single line PEM content to conformant format:
  // add line break after every 64 characters
  private String normalizePEMString(String input) {
    String output = "";
    for (int i = 0; i < input.length(); i += 64) {
      int end = (i + 64) > input.length() ? input.length() : i + 64;
      output += input.substring(i, end);
      output += "\n";
    }
    return output;
  }

  private KeyAndCert getUserKeyAndCert() throws IOException {
    Path userWallet = Paths.get(this.rootdir, this.getEnvironmentId(), this.getMembershipId(), username + ".id");
    String content = Files.readString(userWallet);
    ObjectMapper mapper = new ObjectMapper();
    JsonNode json = mapper.readTree(content);
    JsonNode creds = json.get("credentials");
    String cert = creds.get("certificate").asText();
    String key = creds.get("privateKey").asText();
    Path certFile = Paths.get(userWallet.getParent().toString(), username + ".crt");
    Files.writeString(certFile, cert);
    Path keyFile = Paths.get(userWallet.getParent().toString(), username + ".key");
    Files.writeString(keyFile, key);
    return new KeyAndCert(keyFile.toString(), certFile.toString());
  }
}
