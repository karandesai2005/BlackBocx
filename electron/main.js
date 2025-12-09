const {app, BrowserWindow} = require('electron');
const path = require('path');
function create(){ 
  const w = new BrowserWindow({
    width:900,height:700,
    webPreferences:{
      preload: path.join(__dirname,'preload.js'),
    }
  });
  w.loadFile('index.html');
}
app.whenReady().then(create);
