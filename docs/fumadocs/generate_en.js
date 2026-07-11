const fs = require('fs');
const path = require('path');

function walkDir(dir, callback) {
    fs.readdirSync(dir).forEach(f => {
        let dirPath = path.join(dir, f);
        let isDirectory = fs.statSync(dirPath).isDirectory();
        isDirectory ? walkDir(dirPath, callback) : callback(path.join(dir, f));
    });
}

walkDir('content/docs', (filepath) => {
    if (!filepath.endsWith('.mdx') || filepath.endsWith('.en.mdx')) {
        return;
    }
    const enFilepath = filepath.slice(0, -4) + '.en.mdx';
    if (fs.existsSync(enFilepath)) {
        return;
    }
    
    let content = fs.readFileSync(filepath, 'utf-8');
    content = content.replace(/^title:\s*(.*)$/gm, 'title: [EN] $1');
    fs.writeFileSync(enFilepath, content, 'utf-8');
    console.log(`Created ${enFilepath}`);
});
