'use strict';

const fs = require('fs-extra');
const { join } = require('path');
const { KEYUTIL } = require('jsrsasign');
const YAML = require('js-yaml');
const FabricCAClient = require('fabric-ca-client');
const { Wallets } = require('fabric-network');
const Key = require('fabric-common/lib/impl/ecdsa/key');

class UserWallet {
  constructor(userId) {
    this.userId = userId;
  }

  async signCert(csrPEM, secret, caUrl) {
    const caCertPEM = fs.readFileSync(join(__dirname, 'resources/kaleido-ca.pem')).toString();
    const caClient = new FabricCAClient(`${caUrl}:443`, { trustedRoots: [caCertPEM], verify: false });
    const result = await caClient.enroll({
      enrollmentID: this.userId,
      enrollmentSecret: secret,
      csr: csrPEM
    });
    return result.certificate;
  }

  async createMSPDir(rootdir, caCertPEM, keyPEM, certPEM, membershipId) {
    fs.ensureDirSync(join(rootdir, 'msp/cacerts'));
    fs.ensureDirSync(join(rootdir, 'msp/tlscacerts'));
    fs.ensureDirSync(join(rootdir, 'msp/signcerts'));
    fs.ensureDirSync(join(rootdir, 'msp/keystore'));
    fs.ensureDirSync(join(rootdir, 'tls'));
    fs.writeFileSync(join(rootdir, 'msp/cacerts/ca.pem'), caCertPEM);
    fs.writeFileSync(join(rootdir, 'msp/tlscacerts/ca.pem'), caCertPEM);
    fs.writeFileSync(join(rootdir, 'msp/signcerts/cert.pem'), certPEM);
    fs.writeFileSync(join(rootdir, 'tls/ca.crt'), caCertPEM);
    const keyObj = KEYUTIL.getKey(keyPEM);
    const key = new Key(keyObj);
    const keyFilename = `${key.getSKI()}_sk`;
    fs.writeFileSync(join(rootdir, 'msp/keystore', keyFilename), keyPEM);
    const config = await YAML.load(fs.readFileSync(join(__dirname, 'resources/core.yaml')));
    config.peer.tls.enabled = true;
    config.peer.localMspId = membershipId;
    const yaml = YAML.dump(config, { noRefs: true });
    fs.writeFileSync(join(rootdir, 'core.yaml'), yaml);
    fs.copyFileSync(join(__dirname, 'resources/config.yaml'), join(rootdir, 'msp/config.yaml'));
  }

  async createUserWallet(rootdir, keyPEM, certPEM, membershipId) {
    const wallet = await Wallets.newFileSystemWallet(rootdir);
    const userIdentity = await wallet.get(this.userId);
    if (!userIdentity) {
      const identity = {
        credentials: {
          certificate: certPEM,
          privateKey: keyPEM
        },
        mspId: membershipId,
        type: 'X.509'
      };
      await wallet.put(this.userId, identity);
      console.log(`Created user wallet ${this.userId} and saved to ${rootdir}`);
    } else {
      console.log(`Loaded user wallet ${this.userId} from disk`);
    }
    return wallet;
  }
}

module.exports = UserWallet;