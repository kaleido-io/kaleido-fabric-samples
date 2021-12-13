package com.kaleido.samples.fabric;

import java.io.IOException;
import java.nio.file.Paths;
import java.util.concurrent.TimeoutException;

import com.kaleido.samples.kaleido.Config;
import com.kaleido.samples.kaleido.BusinessNetwork;

import org.hyperledger.fabric.gateway.Contract;
import org.hyperledger.fabric.gateway.ContractException;
import org.hyperledger.fabric.gateway.Gateway;
import org.hyperledger.fabric.gateway.Network;
import org.hyperledger.fabric.gateway.Wallet;

public class Client {
  private Wallet wallet;
  private BusinessNetwork businessNetwork;

  private Contract contract;

  public Client(Wallet wallet, BusinessNetwork network) {
    this.wallet = wallet;
    this.businessNetwork = network;
  }

  public void connect() throws IOException {
    Config config = new Config();
    String username = config.getUsername();
    String chaincode = config.getContractName();
    String ccpFile = this.businessNetwork.getConnectionProfilePath();
    String channel = this.businessNetwork.getTargetChannel();

    Gateway.Builder builder = Gateway.createBuilder();
    // IMPORTANT: must disable discovery. The current Fabric discovery service has a
    // naive assumption that the orderer endpoints in the channel configuration,
    // which are meant for the Fabric peers to use to replicate blocks, are publicly
    // accessile from the client apps. This assumption is invalid in Kaleido's
    // networking setup, where the orderer endpoints are internal to the p2p network
    // among the Fabric nodes.
    //
    // Instead, we rely on the specific configuration in the connection profile to
    // provide the endpoint information for the orderers
    builder.identity(this.wallet, username).networkConfig(Paths.get(ccpFile)).discovery(false);
    Gateway gateway = builder.connect();
    Network network = gateway.getNetwork(channel);
    this.contract = network.getContract(chaincode);
  }

  public void initLedger() throws ContractException, TimeoutException, InterruptedException {
    System.out.println("Submit Transaction: InitLedger creates the initial set of assets on the ledger.");
    this.contract.submitTransaction("InitLedger");
  }

  public void getAllAssets() throws ContractException {
    byte[] result = contract.evaluateTransaction("GetAllAssets");
    System.out.println("Evaluate Transaction: GetAllAssets, result: " + new String(result));
  }

  public void createAsset(String assetId) throws ContractException, TimeoutException, InterruptedException {
    System.out.println("Submit Transaction: CreateAsset " + assetId);
    contract.submitTransaction("CreateAsset", assetId, "yellow", "5", "Tom", "1300");
  }

  public void readAsset(String assetId) throws ContractException {
    System.out.println("Evaluate Transaction: ReadAsset " + assetId);
    byte[] result = contract.evaluateTransaction("ReadAsset", assetId);
    System.out.println("result: " + new String(result));
  }
}
