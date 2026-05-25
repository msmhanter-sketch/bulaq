// Predefined Code Examples
const examples = {
    hello: `"Сәлем, Әлем!" жазу`,
    
    math: `# Математикалық амалдар (SOV синтаксисі)
нәтиже 5 10 қосу 3 көбейту болсын
# Нәтиже: (5 + 10) * 3 = 45
нәтиже жазу`,
    
    loop: `# әзірше циклі арқылы 1-ден 5-ке дейін санау
санауыш 1 болсын

әзірше санауыш кіші_тең 5 {
    санауыш жазу
    санауыш санауыш қосу 1 болсын
}`,
    
    func: `# Екі санды қосатын функция анықтау және шақыру
функция қосу_екі(сан1, сан2) {
    сан1 сан2 қосу қайтару
}

нәтиже қосу_екі(25, 35) болсын
нәтиже жазу`,
    
    struct: `# Құрылымды жариялау және қолдану
құрылым Адам {
    аты МӘТІН
    жасы БҮТІН
}

әли жасау Адам болсын
әли.аты "Әлихан" болсын
әли.жасы 21 болсын

әли.аты жазу
әли.жасы жазу`
};

// UI Elements
const editor = document.getElementById('code-editor');
const lineNumbers = document.getElementById('line-numbers');
const runBtn = document.getElementById('run-btn');
const examplesDropdown = document.getElementById('examples-dropdown');
const compilerStatus = document.getElementById('compiler-status');

const outputConsole = document.getElementById('output-console');
const llvmConsole = document.getElementById('llvm-console');
const nasmConsole = document.getElementById('nasm-console');

// Load selected example
examplesDropdown.addEventListener('change', (e) => {
    editor.value = examples[e.target.value] || '';
    updateLineNumbers();
});

// Update Line Numbers
function updateLineNumbers() {
    const lines = editor.value.split('\n');
    lineNumbers.innerHTML = Array(lines.length).fill(0).map((_, i) => i + 1).join('<br>');
}

editor.addEventListener('input', updateLineNumbers);
editor.addEventListener('scroll', () => {
    lineNumbers.scrollTop = editor.scrollTop;
});

// Setup Tabs
document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
        
        btn.classList.add('active');
        const tabId = btn.getAttribute('data-tab') + '-pane';
        document.getElementById(tabId).classList.add('active');
    });
});

// WASM Integration
const go = new Go();
WebAssembly.instantiateStreaming(fetch('butaq.wasm'), go.importObject).then((result) => {
    go.run(result.instance);
    compilerStatus.textContent = 'Дайын';
    compilerStatus.classList.add('ready');
    
    // Set default value
    editor.value = examples.hello;
    updateLineNumbers();
}).catch(err => {
    console.error('WASM loading error:', err);
    compilerStatus.textContent = 'Қате';
    compilerStatus.style.color = '#ef4444';
});

// Run Code
runBtn.addEventListener('click', () => {
    if (compilerStatus.textContent !== 'Дайын') {
        alert('Компилятор әлі жүктелуде, сәл күте тұрыңыз...');
        return;
    }

    const code = editor.value;

    // 1. Run Interpreter
    outputConsole.classList.remove('error');
    outputConsole.textContent = 'Орындалуда...';
    
    const interpreterResult = window.runButaqCode(code);
    if (interpreterResult.error) {
        outputConsole.classList.add('error');
        outputConsole.textContent = interpreterResult.error;
    } else {
        outputConsole.textContent = interpreterResult.output || 'Бағдарлама сәтті аяқталды (шығыс мәліметтер жоқ)';
    }

    // 2. Compile to LLVM IR
    const llvmResult = window.compileToLlvm(code);
    if (llvmResult.error) {
        llvmConsole.textContent = llvmResult.error;
    } else {
        llvmConsole.textContent = llvmResult.code;
    }

    // 3. Compile to NASM
    const nasmResult = window.compileToNasm(code);
    if (nasmResult.error) {
        nasmConsole.textContent = nasmResult.error;
    } else {
        nasmConsole.textContent = nasmResult.code;
    }
});
