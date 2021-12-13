package com.kaleido.samples.fabric;

import java.io.IOException;
import java.lang.reflect.InvocationTargetException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpRequest.BodyPublishers;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.security.cert.CertificateException;
import java.util.Properties;
import java.net.http.HttpResponse;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.kaleido.samples.kaleido.Config;
import com.kaleido.samples.kaleido.BusinessNetwork;

import org.hyperledger.fabric.gateway.Identities;
import org.hyperledger.fabric.gateway.Identity;
import org.hyperledger.fabric.gateway.Wallet;
import org.hyperledger.fabric.gateway.Wallets;
import org.hyperledger.fabric.sdk.Enrollment;
import org.hyperledger.fabric.sdk.exception.CryptoException;
import org.hyperledger.fabric.sdk.exception.InvalidArgumentException;
import org.hyperledger.fabric.sdk.security.CryptoSuite;
import org.hyperledger.fabric.sdk.security.CryptoSuiteFactory;
import org.hyperledger.fabric_ca.sdk.EnrollmentRequest;
import org.hyperledger.fabric_ca.sdk.HFCAClient;
import org.hyperledger.fabric_ca.sdk.exception.EnrollmentException;

public class ClientWallet {
  private BusinessNetwork network;
  private Config config;
  private Wallet wallet;

  public ClientWallet(BusinessNetwork network) {
    this.network = network;
    this.config = new Config();
  }

  public Wallet getWallet() {
    return this.wallet;
  }

  public void ensureUserInWallet()
      throws IOException, CryptoException, InvalidArgumentException, ClassNotFoundException, IllegalAccessException,
      InstantiationException, NoSuchMethodException, InvocationTargetException, EnrollmentException,
      org.hyperledger.fabric_ca.sdk.exception.InvalidArgumentException, CertificateException, InterruptedException {
    // the Fabric SDK for Java requires an "admin" named user to exist in the wallet
    // before other users can be enrolled
    this.ensureIdentityInWallet("admin", "admin-local", "tls");
    String username = this.config.getUsername();
    this.ensureIdentityInWallet(username, username, "");
  }

  private void ensureIdentityInWallet(String username, String enrollmentId, String profile)
      throws IOException, CryptoException, InvalidArgumentException, ClassNotFoundException, IllegalAccessException,
      InstantiationException, NoSuchMethodException, InvocationTargetException, EnrollmentException,
      org.hyperledger.fabric_ca.sdk.exception.InvalidArgumentException, CertificateException, InterruptedException {
    if (this.wallet == null) {
      // the wallet is per environment, per membership
      Path walletDir = Paths.get(this.config.getRootDir(), this.network.getEnvironmentId(),
          this.network.getMembershipId());
      this.wallet = Wallets.newFileSystemWallet(walletDir);
    }
    Identity user = wallet.get(username);
    if (user != null) {
      System.out.println(String.format("User %s already exists in the wallet", username));
      return;
    }
    System.out.println(String.format("User %s does not exist. Will register and enroll", username));
    this.enrollUser(username, enrollmentId, profile);
  }

  public void enrollUser(String username, String enrollmentId, String profile)
      throws IOException, InterruptedException, CryptoException,
      InvalidArgumentException, ClassNotFoundException, IllegalAccessException, InstantiationException,
      NoSuchMethodException, InvocationTargetException, EnrollmentException,
      org.hyperledger.fabric_ca.sdk.exception.InvalidArgumentException, CertificateException {
    // we first call the Kaleido endpoint to register the user with the
    // Fabric CA of the Kaleido membership
    String rootUrl = this.config.getKaleidoUrl();
    String caId = this.network.getCAServiceId();

    String payload = String.format("{\"registrations\": [{\"enrollmentID\": \"%s\", \"role\": \"admin\"}]}",
        enrollmentId);

    String url = String.format("%s/fabric-ca/%s/register", rootUrl, caId);
    HttpRequest request = HttpRequest.newBuilder()
        .uri(URI.create(url))
        .POST(BodyPublishers.ofString(payload))
        .header("Authorization", "Bearer " + this.config.getAPIKey())
        .header("Content-Type", "application/json")
        .build();
    HttpClient client = HttpClient.newHttpClient();
    HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());

    ObjectMapper mapper = new ObjectMapper();
    JsonNode result = mapper.readTree(response.body());
    JsonNode registrations = result.get("registrations");
    if (registrations == null) {
      JsonNode errMsg = result.get("errorMessage");
      if (errMsg != null) {
        System.out.println(String.format("Attempt to register user with Fabric CA failed: %s", errMsg.asText()));
        System.exit(1);
      }
    }
    String enrollmentSecret = registrations.get(0).get("enrollmentSecret").asText();
    System.out.println(
        String.format("Successfully registered user \"%s\" using enrollmentId \"%s\"", username, enrollmentId));

    Properties props = new Properties();
    props.put("pemFile", this.network.getKaleidoTLSCert());
    props.put("allowAllHostNames", "true");
    HFCAClient caClient = HFCAClient.createNewInstance(this.network.getCAUrl(),
        props);
    CryptoSuite cryptoSuite = CryptoSuiteFactory.getDefault().getCryptoSuite();
    caClient.setCryptoSuite(cryptoSuite);

    final EnrollmentRequest enrollmentRequest = new EnrollmentRequest();
    if (!profile.equals("")) {
      enrollmentRequest.setProfile(profile);
    }

    Enrollment enrollment = caClient.enroll(enrollmentId, enrollmentSecret, enrollmentRequest);
    Identity user = Identities.newX509Identity(this.network.getMembershipId(), enrollment);
    this.wallet.put(username, user);
    System.out.println(String.format("Successfully enrolled user \"%s\" and imported it into the wallet", username));
  }
}
