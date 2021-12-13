/*
 * This Java source file was generated by the Gradle 'init' task.
 */
package com.kaleido.samples;

import java.io.IOException;
import java.lang.reflect.InvocationTargetException;
import java.security.cert.CertificateEncodingException;
import java.security.cert.CertificateException;
import java.util.Random;
import java.util.Scanner;
import java.util.concurrent.TimeoutException;

import com.kaleido.samples.fabric.Client;
import com.kaleido.samples.fabric.ClientWallet;
import com.kaleido.samples.kaleido.BusinessNetwork;

import org.apache.commons.codec.DecoderException;
import org.hyperledger.fabric.gateway.ContractException;
import org.hyperledger.fabric.sdk.exception.CryptoException;
import org.hyperledger.fabric.sdk.exception.InvalidArgumentException;
import org.hyperledger.fabric_ca.sdk.exception.EnrollmentException;

public class App {
    public static void main(String[] args)
            throws IOException, InterruptedException, DecoderException, CryptoException,
            InvalidArgumentException, ClassNotFoundException, IllegalAccessException, InstantiationException,
            NoSuchMethodException, InvocationTargetException, EnrollmentException,
            org.hyperledger.fabric_ca.sdk.exception.InvalidArgumentException, CertificateException, ContractException,
            TimeoutException {

        Scanner input = new Scanner(System.in);

        // use Kaleido API to build the Fabric network connection profile
        BusinessNetwork kaleidoNetwork = new BusinessNetwork(input);
        kaleidoNetwork.build();

        // ensure the client Identity is created in the wallet
        ClientWallet wallet = new ClientWallet(kaleidoNetwork);
        wallet.ensureUserInWallet();

        // persist the connection profile, which requires the user to have been enrolled
        // first
        // in order to use the key and cert for mutual TLS client cert and key settings
        kaleidoNetwork.writeConnectionProfile();

        // ready to interact with the Fabric network
        Client client = new Client(wallet.getWallet(), kaleidoNetwork);
        client.connect();

        // whether init ledger needs to be called is a decision by the contract deployer
        // so we must ask the user here
        System.out.print("Call \"InitLedger\"? (y/n) ");
        String str = input.nextLine();
        if (str.toUpperCase().equals("Y")) {
            client.initLedger();
        }

        int rand = new Random().nextInt(1000 * 1000);
        client.createAsset("asset-" + rand);
        client.getAllAssets();

        input.close();
    }
}