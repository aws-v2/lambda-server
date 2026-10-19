const helper = require('./helper'); const payload = JSON.parse(process.env.PAYLOAD || '{}'); console.log(helper.greet(payload.name || 'Zip'));
