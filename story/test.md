# Testing prompts

Gebruik de volgende prompts om de verschillende mogelijkheden te testen.
  
- [x] VIDEO: maak een kinder film over het zuiveren van afval water, 1 min, youtube
- [x] CHAT: Kan je mij een verhaal vertellen over druppie
- [x] CHAT vervolgstap: Vertel meer over het Druppie-platform.
- [x] BUILD NodeJs: Maak een commandline node.js hello world script, bouw het en draai het.
- [x] BUILD Python: Maak een commandline python hello world script, bouw het en draai het.
- [x] BUILD Go: Maak een commandline go hello world script, bouw het en draai het.
- [x] COMPLIANCE: Ik wil een nieuwe publieke zorg database op AWS us-east-1 deployen met patiënt gegevens
- [x] MCP: Maak een file met MCP genaamd mcp_test.txt met content 'Hello MCP' in de huidige directory
- [x] PLUGIN: Create a new Node.js MCP plugin named 'calc-mcp'. It MUST NOT use any external dependencies or SDKs. It MUST implement a simple JSON-RPC 2.0 server using the 'readline' module. It must support 3 methods: 1) 'initialize' (returning capabilities), 2) 'tools/list' (returning an 'add' tool definition accepting {a, b}), and 3) 'tools/call' (implementing the logic: return a + b). The package.json must have a 'bin' object mapping 'calc-mcp' to './index.js'. Test the plugin, and if successful, promote it.
- [x] PLUGIN:Create a new MCP plugin named 'calc-mcp' with an 'add' tool that sums two numbers (a, b). Test and promote it.
- [x] MCP : use plugin calc-mcp to add 5 and 11