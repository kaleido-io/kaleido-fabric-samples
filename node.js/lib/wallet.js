/*
 * Copyright 2021 Kaleido
 * 
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 * 
 *     http://www.apache.org/licenses/LICENSE-2.0
 * 
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
'use strict';

const { join } = require('path');
const { KEYUTIL, KJUR } = require('jsrsasign');
const FabricCAClient = require('fabric-ca-client');
const { Wallets } = require('fabric-network');

class UserWallet {
  constructor(rootdir, membershipId) {
    this.rootdir = rootdir;
    this.membershipId = membershipId;
  }

  async init() {
    this.wallet = await Wallets.newFileSystemWallet(join(this.rootdir, this.membershipId));
  }

  async getUser(userId) {
		// Check to see if we've already enrolled the admin user.
		return await this.wallet.get(userId);
  }

  async newUser(userId, secret, caUrl) {
    const { csrPEM, keyPEM } = await this.generateCSR(userId);
    const caClient = new FabricCAClient(`${caUrl}:443`, { verify: false });
    const result = await caClient.enroll({
      enrollmentID: userId,
      enrollmentSecret: secret,
      csr: csrPEM
    });
    const certPEM = result.certificate;
    const identity = {
      credentials: {
        certificate: certPEM,
        privateKey: keyPEM
      },
      mspId: this.membershipId,
      type: 'X.509'
    };
    await this.wallet.put(userId, identity);
    console.log(`Successfully enrolled user ${userId} and saved to ${this.rootdir}`);
    return identity;
  }

  async generateCSR(userId) {
    let subject = `/C=US/L=Raleigh/O=Kaleido/OU=admin/CN=${userId}`;
    const alg = 'EC';
    const keylenOrCurve = "secp256r1";
    const sigalgName = 'SHA256withECDSA';
    let keypair = KEYUTIL.generateKeypair(alg, keylenOrCurve);
  
    let options = {
      sigalg: sigalgName,
      subject: {str: subject},
      sbjpubkey: keypair.pubKeyObj,
      sbjprvkey: keypair.prvKeyObj
    };
  
    let csrPEM = KJUR.asn1.csr.CSRUtil.newCSRPEM(options);
    let keyPEM = KEYUTIL.getPEM(keypair.prvKeyObj, "PKCS8PRV");
  
    return { csrPEM, keyPEM };
  }
}

module.exports = UserWallet;