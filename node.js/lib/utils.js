'use strict';

function handleOutput(cmd) {
  return new Promise((resolve, reject) => {
    let consoleData = '';
    cmd.stdout.on('data', data => {
      consoleData += data.toString();
    });

    cmd.stderr.on('data', data => {
      consoleData += data.toString();
    });

    cmd.on('error', err => {
      console.error(err, cmd.spawnargs);
      reject(err);
    });

    cmd.on('close', code => {
      console.info(`Command exitted with code ${code}`, cmd.spawnargs);
      console.info(consoleData);
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`Command exitted with non-zero code: ${code}`));
      }
    });
  });
}

module.exports = {
  handleOutput
};