const helper = require("./helper"); module.exports = { handle: (event) => { return { message: helper.greet(event.name || "Zip") }; } };
