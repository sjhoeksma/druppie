# Druppie UI Copilot Integratie Onderzoek

**Uitgebreid Technisch Onderzoeksdocument**  
*Versie 2.0 — December 2024*

---

## Onderzoeksvraag

**Hoe kunnen we Druppie integreren in Microsoft 365 Copilot om gebruikers toegang te geven tot het Druppie multi-agent systeem via de Copilot-interface?**

Dit document presenteert uitgebreid onderzoek naar de integratie van het Druppie multi-agent systeem met Microsoft 365 Copilot, Teams en gerelateerde diensten. Om deze vraag grondig te beantwoorden, moeten we eerst het Microsoft-ecosysteem begrijpen en de verschillende benaderingen die beschikbaar zijn voor het bouwen van agents. Dit onderzoek is gestructureerd rond verschillende kennisdeelvragen die leiden naar onze uiteindelijke aanbeveling.

---

## samenvatting

Microsoft biedt drie verschillende benaderingen voor het uitbreiden van Copilot met aangepaste agents. Na uitgebreide analyse bevelen we de **M365 Agents SDK in combinatie met de Custom Engine Agent-benadering** aan voor de architectuur van Druppie. Deze aanbeveling is gebaseerd op Druppie's vereiste voor volledige orchestratiecontrole, de mogelijkheid om aangepaste AI-modellen te gebruiken, en de noodzaak om een draagbaar, zelf-gehost kernsysteem te behouden.

| Aspect | Aanbeveling |
|--------|-------------|
| **Integratiemethode** | M365 Agents SDK + Custom Engine Agent |
| **Architectuur** | Thin Client (Azure) + Core Druppie (On-premises) |
| **Authenticatie** | Entra ID SSO → Keycloak federatie |
| **Toestemmingsmodel** | Per-aanroep toestemming voor schrijfoperaties |
| **SDK Licentie** | MIT (volledig open source) |
| **Ontwikkeltools** | M365 Agents Toolkit (VS Code / Visual Studio) |

De fundamentele beslissing komt neer op een afweging tussen **eenvoud en controle**: declaratieve agents bieden snellere time-to-value met door Microsoft beheerde infrastructuur, terwijl custom engine agents maximale flexibiliteit bieden tegen de kosten van grotere ontwikkelings- en operationele complexiteit. Voor Druppie maakt de behoefte aan aangepaste orchestratie de custom engine-benadering essentieel.

---

## Deel 1: Het Microsoft-ecosysteem Begrijpen

Voordat we ingaan op specifieke technologieën, is het essentieel om het Microsoft-ecosysteem en de gebruikte terminologie te begrijpen. Deze sectie biedt fundamentele kennis voor lezers die mogelijk niet bekend zijn met Microsoft's AI- en samenwerkingsplatforms.

### Wat is Microsoft 365 Copilot?

Microsoft 365 Copilot is Microsoft's AI-assistent die is ingebouwd in hun productiviteitssuite (Word, Excel, PowerPoint, Outlook, Teams). Zie het als ChatGPT, maar diep geïntegreerd in de tools die miljoenen mensen dagelijks voor hun werk gebruiken. Wanneer een gebruiker een vraag typt in de Copilot-chatinterface, verwerkt Microsoft's AI deze, doorzoekt mogelijk de e-mails, documenten en agenda van de gebruiker, en geeft een intelligent antwoord.

Het belangrijkste inzicht is dat Copilot **uitbreidbaar** is. Microsoft staat ontwikkelaars toe om "agents" te bouwen die de mogelijkheden van Copilot uitbreiden. In plaats van alleen algemene vragen te beantwoorden, kan een uitgebreide Copilot interageren met uw specifieke bedrijfssystemen, databases en workflows. Dit is waar Druppie-integratie mogelijk wordt.

### Belangrijke Terminologie

Het begrijpen van deze termen is essentieel voor het volgen van de rest van dit document:

**Copilot** verwijst naar Microsoft's AI-assistent die is ingebed in M365-apps. Het dient als de gebruikersgerichte interface waar mensen vragen stellen en AI-gestuurde antwoorden ontvangen.

**Agent** is een gespecialiseerde AI-applicatie die taken kan uitvoeren, vragen kan beantwoorden en kan interageren met externe systemen. Agents breiden uit wat Copilot kan doen door nieuwe mogelijkheden toe te voegen.

**Orchestrator** is het "brein" dat AI-redenering coördineert, beslist welke tools te gebruiken en de gespreksstroom beheert. Dit is waar de daadwerkelijke AI-besluitvorming plaatsvindt.

**Azure Bot Service** is Microsoft's cloudinfrastructuur voor het routeren van berichten tussen gebruikers en agent-code. Het handelt de communicatie-infrastructuur af zodat ontwikkelaars zich kunnen richten op bedrijfslogica.

**Entra ID** (voorheen Azure Active Directory) is Microsoft's identiteitsdienst voor authenticatie. Het verifieert wie gebruikers zijn en waar ze toegang toe hebben.

**MCP (Model Context Protocol)** is een open standaard voor het verbinden van AI-systemen met externe tools en databronnen. Het biedt een gestandaardiseerde manier voor AI om te interageren met databases, API's en andere systemen.

**Activity** is een gestructureerd bericht dat wordt uitgewisseld tussen gebruikers en agents. Activities kunnen tekstberichten, bestandsuploads, knopklikken of systeemgebeurtenissen zijn.

**Turn** vertegenwoordigt één complete communicatieronde: de gebruiker stuurt een bericht, de agent verwerkt het, en de agent reageert. Staatsbeheer vindt plaats tussen turns.

**TurnContext** is het object dat alle informatie over de huidige turn bevat, inclusief het inkomende bericht, methoden om antwoorden te verzenden en toegang tot de gespreksstatus.

---

## Kennisdeelvraag 1: Wat zijn Declaratieve Agents?

Declaratieve agents zijn aangepaste versies van Microsoft 365 Copilot die zijn gemaakt door middel van **configuratie in plaats van code**. Ze draaien op Microsoft's orchestrator en foundation AI-modellen en erven automatisch alle beveiliging, compliance en Responsible AI-beschermingen. Zie ze als het geven van een gespecialiseerde "persona" aan Copilot met specifieke instructies en toegang tot bepaalde kennisbronnen.

### Hoe Declaratieve Agents Werken

De architectuur draait om een **declaratief agent-manifest**—een JSON-configuratiebestand dat het gedrag van de agent definieert. Je schrijft geen code om AI-redenering af te handelen; in plaats daarvan beschrijf je wat je wilt dat de agent doet, en Microsoft's infrastructuur handelt de rest af.

Het manifest bevat verschillende belangrijke componenten:

**Instructies** (tot 8.000 tekens) zijn gedragsrichtlijnen die bepalen hoe de agent reageert. Je kunt bijvoorbeeld een IT-helpdesk agent instrueren om altijd verduidelijkende vragen te stellen voordat oplossingen worden geboden, of om antwoorden op een specifieke manier te formatteren.

**Kennisbronnen** definiëren welke informatie de agent kan raadplegen. Ondersteunde bronnen zijn SharePoint-sites, OneDrive-mappen, Graph-connectors (100+ voorgebouwde connectors naar bedrijfssystemen), Teams-berichten, e-mailarchieven en webzoekopdrachten met site-scoping.

**Acties** zijn verbindingen met externe systemen via API-plugins met OpenAPI-specificaties. Wanneer een gebruiker de agent vraagt om een actie uit te voeren (zoals het aanmaken van een ticket of het controleren van voorraad), roept de agent deze API's aan.

**Ingebouwde mogelijkheden** omvatten Code Interpreter voor het uitvoeren van Python-code en GraphicArt voor het genereren van afbeeldingen.

### Declaratieve Agents Maken

Microsoft biedt drie paden voor het maken van declaratieve agents, elk gericht op verschillende vaardigheidsniveaus:

**No-code** creatie gebeurt via SharePoint Agent Builder of de Copilot Agent Builder-interface. Gebruikers beschrijven simpelweg in natuurlijke taal wat ze willen, selecteren kennisbronnen, en Microsoft genereert de configuratie.

**Low-code** creatie gebruikt Copilot Studio, Microsoft's visuele ontwikkelomgeving. Dit biedt meer controle dan no-code maar vereist geen traditionele programmeervaardigheden.

**Pro-code** creatie gebruikt de M365 Agents Toolkit in Visual Studio Code of Visual Studio. Ontwikkelaars bewerken JSON-manifesten direct en kunnen integreren met bronbeheer en CI/CD-pipelines.

### Wanneer Declaratieve Agents te Gebruiken

Declaratieve agents excelleren in scenario's waar Microsoft's ingebouwde AI-mogelijkheden voldoende zijn:

Ze werken goed voor **IT-helpdesk agents** die vragen beantwoorden op basis van interne documentatie, **onboarding-assistenten voor medewerkers** die nieuwe medewerkers door bedrijfsbeleid leiden, **documentsamenvatting-tools** die gebruikers helpen complexe rapporten te begrijpen, en **klantenservice-agents** die kennisbanken gebruiken om veelgestelde vragen te beantwoorden.

### Beperkingen van Declaratieve Agents

De belangrijkste beperkingen zijn architecturaal—ze vertegenwoordigen ontwerpkeuzes, geen bugs die moeten worden opgelost:

**Geen aangepaste AI-modellen**: Je kunt Microsoft's foundation-modellen niet verwisselen voor je eigen fijn-afgestemde modellen, Claude of andere AI-providers. De orchestrator ligt vast.

**Geen proactieve berichten**: De agent kan alleen reageren op gebruikersvragen; hij kan geen gesprekken initiëren op basis van externe gebeurtenissen (zoals het sturen van een notificatie wanneer een build voltooid is).

**Geen externe uitrol**: Declaratieve agents werken alleen binnen Microsoft 365-applicaties. Je kunt ze niet inbedden in je eigen website of mobiele app.

**Instructielimieten**: De limiet van 8.000 tekens voor instructies kan onvoldoende zijn voor complexe gedragsspecificaties.

**Rechten-overerving**: Agents kunnen alleen toegang krijgen tot content waarvoor de gebruiker al toestemming heeft. Dit is een beveiligingsfunctie, maar het betekent dat agents geen verhoogde operaties kunnen uitvoeren.

### Waarom Declaratieve Agents Niet Geschikt Zijn voor Druppie

Druppie vereist **aangepaste orchestratie** met behulp van OpenCode en meerdere gespecialiseerde sub-agents (SPEC Agent, Code Agent, Review Agent, Security Agent). Declaratieve agents bieden geen mechanisme voor dit soort multi-agent coördinatie. Bovendien moet Druppie verbinding maken met on-premises systemen (Gitea, DataLab) met aangepaste authenticatiestromen die declaratieve agents niet kunnen accommoderen.

---

## Kennisdeelvraag 2: Wat zijn Custom Engine Agents?

Custom engine agents vertegenwoordigen een fundamenteel andere architectuur waarbij **ontwikkelaars het AI-"brein" controleren**. In plaats van Microsoft's orchestrator te gebruiken, breng je je eigen mee—of dat nu Semantic Kernel, LangChain, Azure AI Foundry of volledig aangepaste logica is. De agent draait op jouw infrastructuur, en jij bepaalt welke AI-modellen je gebruikt.

### Hoe Custom Engine Agents Werken

Het belangrijkste architecturale onderscheid is wie de orchestratie controleert:

Bij declaratieve agents handelt Microsoft's orchestrator prompt engineering, function calling en redeneringsflow af. Jouw configuratie vertelt wat te doen, maar Microsoft's code doet het daadwerkelijke werk.

Bij custom engine agents **handelt jouw code alles af**. Microsoft biedt de communicatie-infrastructuur (Azure Bot Service) om berichten te routeren tussen de Copilot-interface en jouw agent, maar wat er binnen jouw agent gebeurt, is volledig aan jou.

Dit betekent dat je kunt:

**Elk AI-model gebruiken**: Azure OpenAI, Anthropic Claude, Google Gemini, Llama, Mistral of je eigen fijn-afgestemde modellen. Je kunt zelfs verschillende modellen gebruiken voor verschillende taken binnen dezelfde agent.

**Aangepaste orchestratie implementeren**: Gebruik Semantic Kernel voor geavanceerde multi-stap redenering, LangChain voor retrieval-augmented generation, of je eigen orchestratielogica.

**Proactieve berichten ondersteunen**: Jouw agent kan gesprekken initiëren op basis van externe gebeurtenissen. Wanneer een CI/CD-pipeline faalt, kan jouw agent de relevante ontwikkelaar notificeren zonder te wachten tot ze ernaar vragen.

**Naar meerdere kanalen deployen**: Dezelfde agent-code kan Microsoft 365 Copilot, Microsoft Teams, je bedrijfswebsite, SMS, Slack en andere platforms gelijktijdig bedienen.

**Multi-user samenwerking mogelijk maken**: Custom engine agents kunnen werken in Teams-kanalen met meerdere deelnemers, waarbij context over een groepsgesprek wordt behouden.

### Architectuurvergelijking

| Mogelijkheid | Declaratieve Agents | Custom Engine Agents |
|--------------|---------------------|---------------------|
| AI-modellen | Alleen Microsoft foundation-modellen | Elk model naar keuze |
| Orchestratie | Door Microsoft beheerd | Door ontwikkelaar gecontroleerd |
| Hosting vereist | Nee | Ja |
| Proactieve berichten | Niet ondersteund | Volledig ondersteund |
| Multi-user samenwerking | Alleen individueel gebruik | Groepsproductiviteit in Teams-kanalen |
| Deployment-kanalen | Alleen M365-apps | M365 + externe websites, portals, aangepaste apps |
| Compliance/Beveiliging | Geërfd van M365 | Verantwoordelijkheid ontwikkelaar |
| Ontwikkelcomplexiteit | Laag (configuratie) | Hoog (volledige ontwikkeling) |

### Wanneer Custom Engine Agents te Gebruiken

Custom engine agents zijn de juiste keuze wanneer je vereisten verder gaan dan wat declaratieve agents kunnen bieden:

**Domeinspecifieke AI-modellen** zijn nodig wanneer algemene modellen niet goed presteren op je gespecialiseerde content. Gezondheidszorg-, juridische en financiële applicaties vereisen vaak fijn-afgestemde modellen getraind op domeinspecifieke data.

**Complexe multi-stap workflows** vereisen aangepaste orchestratie wanneer de bedrijfslogica conditionele vertakkingen, parallelle verwerking of coördinatie tussen meerdere AI-systemen omvat.

**Proactieve automatisering** maakt event-gedreven scenario's mogelijk waarbij de agent actie initieert op basis van externe triggers in plaats van te wachten op gebruikersvragen.

**Externe distributie** is vereist wanneer je je agent wilt publiceren naar klantportals, partnerwebsites of de Microsoft Commercial Store.

**Bestaande AI-investeringen** moeten worden benut wanneer je al AI-mogelijkheden hebt gebouwd met frameworks zoals LangChain, Semantic Kernel of aangepaste orchestratie en deze wilt ontsluiten via de Copilot-interface.

### Afwegingen van Custom Engine Agents

De flexibiliteit brengt verantwoordelijkheden met zich mee:

**Infrastructuurbeheer**: Je moet je hosting-infrastructuur provisioneren, monitoren en schalen. Azure App Service, Container Apps of Kubernetes zijn veelvoorkomende keuzes.

**Beveiligingsimplementatie**: Je bent verantwoordelijk voor gegevensbescherming, inputvalidatie en veilige afhandeling van gevoelige informatie.

**Compliance-eigenaarschap**: In tegenstelling tot declaratieve agents waar Microsoft compliance afhandelt, vereisen custom engine agents dat jij compliance met relevante regelgeving implementeert en certificeert.

**Hogere ontwikkelinspanning**: Het bouwen van een custom engine agent vereist aanzienlijk meer ontwikkeltijd dan het configureren van een declaratieve agent.

### Waarom Custom Engine Agents Juist Zijn voor Druppie

Druppie's architectuur vereist fundamenteel custom engine-mogelijkheden:

Het **OpenCode-gebaseerde multi-agent systeem** met gespecialiseerde sub-agents (SPEC, Code, Review, Security) vereist aangepaste orchestratie die declaratieve agents niet kunnen bieden.

**On-premises componenten** (Core Druppie, Gitea, DataLab) vereisen hybride connectiviteit en aangepaste authenticatiestromen.

**Per-aanroep toestemming** voor schrijfoperaties vereist aangepaste UI-stromen met Adaptive Cards die verder gaan dan de mogelijkheden van declaratieve agents.

**Multi-channel deployment** (Copilot, Teams, Druppie Portal) vereist de flexibiliteit van custom engine agents.

---

## Kennisdeelvraag 3: Wat is de M365 Agents SDK?

De Microsoft 365 Agents SDK is de **runtime-bibliotheek en framework** voor het bouwen van custom engine agents. Het is de directe opvolger van de Bot Framework SDK, volledig opnieuw ontworpen voor het tijdperk van generatieve AI. Microsoft stelt expliciet dat de industrie is geëvolueerd om agents te vereisen die acties kunnen orchestreren, niet alleen vragen beantwoorden.

### De Rol van de SDK Begrijpen

De SDK biedt de fundamentele bouwstenen voor agent-ontwikkeling:

**Kanaalconnectiviteit** handelt de complexiteit af van communiceren met Microsoft 365 Copilot, Teams en andere berichtenplatforms. Elk kanaal heeft zijn eigen protocollen en eigenaardigheden; de SDK abstraheert deze verschillen zodat je code consistent werkt.

**Activity-verwerking** biedt een gestructureerde manier om inkomende berichten en gebeurtenissen af te handelen. De SDK routeert verschillende typen activities (berichten, bestandsuploads, knopklikken, gespreksupdates) naar de juiste handlers in je code.

**Staatsbeheer** persisteert gesprekscontext tussen gebruikersinteracties. De SDK biedt interfaces voor het opslaan van gespreksstatus, gebruikersvoorkeuren en andere gegevens die nodig zijn om coherente multi-turn gesprekken te onderhouden.

**Authenticatie-integratie** verbindt met Microsoft Entra ID voor single sign-on en tokenbeheer. De SDK handelt OAuth-stromen, token-vernieuwing en veilige credential-opslag af.

### Het Activity Protocol en Programmeermodel

De SDK gebruikt een **activity-gebaseerd programmeermodel** waarbij je agent reageert op verschillende typen gebeurtenissen. Activities zijn gestructureerde JSON-objecten die elke interactie tussen een gebruiker en je agent vertegenwoordigen—niet alleen tekstberichten, maar ook bestandsuploads, kaartknopklikken, typindicatoren en systeemgebeurtenissen.

Het belangrijkste object in de SDK is de **TurnContext**, die wordt aangemaakt voor elke turn van het gesprek. Het biedt toegang tot de inkomende activity, methoden voor het verzenden van antwoorden en de huidige gespreksstatus. Hier is hoe het basispatroon werkt:

```csharp
// Registreer een handler voor message activities
agent.OnActivity(ActivityTypes.Message, async (turnContext, turnState, cancellationToken) => 
{
    // Haal het bericht van de gebruiker op
    var userMessage = turnContext.Activity.Text;
    
    // Verwerk het bericht (jouw aangepaste logica komt hier)
    var response = await ProcessWithYourAI(userMessage);
    
    // Stuur het antwoord terug
    await turnContext.SendActivityAsync(MessageFactory.Text(response), cancellationToken);
});
```

Voor JavaScript/TypeScript is het patroon vergelijkbaar:

```javascript
class EchoAgent extends AgentApplication {
    constructor(storage) {
        super({ storage });
        this.onMessage('/help', this._help);
        this.onActivity('message', this._echo);
    }
    
    _echo = async (context, state) => {
        await context.sendActivity(`Je zei: ${context.activity.text}`);
    }
}
```

En voor Python:

```python
@AGENT_APP.activity("message")
async def on_message(context: TurnContext, state: TurnState):
    await context.send_activity(f"Je zei: {context.activity.text}")
```

### Belangrijke SDK-concepten

**Turn**: Een turn is één complete communicatieronde. De gebruiker stuurt een bericht, je agent ontvangt het, verwerkt het en reageert. De TurnContext bestaat alleen voor de duur van een enkele turn en wordt verwijderd wanneer de turn eindigt.

**Activity Types**: De SDK definieert vele activity-typen waaronder Message (tekst en bijlagen), Event (aangepaste gebeurtenissen van kanalen), ConversationUpdate (leden die toetreden/vertrekken), Typing (typindicatoren) en Invoke (commando's en operaties).

**State Storage**: De SDK biedt een IStorage-interface voor het persisteren van staat. Tijdens ontwikkeling houdt MemoryStorage alles in RAM; voor productie gebruik je doorgaans Azure Blob Storage of CosmosDB voor persistentie.

**Route Handlers**: Je registreert handlers voor verschillende activity-typen en patronen. De SDK routeert inkomende activities naar de juiste handler op basis van type, commandopatronen of aangepaste logica.

### Proactieve Berichten

Een van de krachtigste mogelijkheden van de SDK is **proactieve berichten**—de mogelijkheid om berichten naar gebruikers te sturen zonder te wachten tot ze een gesprek initiëren. Dit is essentieel voor notificatiescenario's:

```csharp
// Sla de gespreksreferentie op tijdens een normale turn
var conversationReference = turnContext.Activity.GetConversationReference();

// Later, wanneer een externe gebeurtenis optreedt (bijv. build voltooid):
await adapter.ProcessProactiveAsync(
    identity,
    conversationReference.GetContinuationActivity(),
    null,
    async (proactiveContext, ct) => {
        await proactiveContext.SendActivityAsync("Je build is succesvol voltooid!");
    }
);
```

Deze mogelijkheid stelt Druppie in staat om gebruikers te notificeren wanneer code reviews voltooid zijn, wanneer builds klaar zijn, of wanneer beveiligingsscans problemen detecteren—zonder dat gebruikers handmatig hoeven te controleren.

### OAuth en Authenticatie

De SDK biedt ingebouwde ondersteuning voor OAuth-stromen met Microsoft Entra ID. Je kunt authenticatie configureren op agent-niveau of per route:

```csharp
public class MyAgent : AgentApplication
{
    public MyAgent(AgentApplicationOptions options) : base(options)
    {
        // Registreer handler met automatische authenticatie
        OnActivity(ActivityTypes.Message, OnMessageAsync, rank: RouteRank.Last);
    }
    
    public async Task OnMessageAsync(ITurnContext turnContext, ITurnState turnState, CancellationToken cancellationToken)
    {
        // Haal het token van de gebruiker op voor het aanroepen van downstream API's
        var token = await UserAuthorization.GetTurnTokenAsync(turnContext);
        
        // Gebruik het token om Microsoft Graph of je eigen API's aan te roepen
        var graphClient = CreateGraphClient(token);
        var userProfile = await graphClient.Me.GetAsync();
    }
}
```

Voor On-Behalf-Of (OBO) stromen waarbij je het token van de gebruiker moet uitwisselen voor een met andere scopes:

```csharp
var exchangedToken = await UserAuthorization.ExchangeTurnTokenAsync(
    turnContext, 
    exchangeScopes: new[] { "api://your-api/.default" }
);
```

### SDK Beschikbaarheid

De M365 Agents SDK is volledig open source onder de MIT-licentie, beschikbaar voor drie platforms:

**C# (.NET 8.0)**: De meest volwassen implementatie, algemeen beschikbaar sinds eind 2024. Beschikbaar via NuGet-pakketten en GitHub op https://github.com/Microsoft/Agents-for-net

**JavaScript (Node.js 18+)**: Beschikbaar voor TypeScript- en JavaScript-ontwikkelaars. GitHub: https://github.com/Microsoft/Agents-for-js

**Python (3.9-3.11)**: Python-implementatie voor teams die dat ecosysteem prefereren. GitHub: https://github.com/Microsoft/Agents-for-python

---

## Kennisdeelvraag 4: Wat is de M365 Agents Toolkit?

De M365 Agents Toolkit is de **IDE-extensie en CLI-tooling** voor het bouwen van agents. Terwijl de SDK de runtime-bibliotheek is waarvan je code afhankelijk is, is de Toolkit de ontwikkelomgeving die je helpt agents efficiënt te maken, debuggen en deployen.

### Het Onderscheid Begrijpen

Een veelvoorkomend punt van verwarring is het verschil tussen de SDK en de Toolkit:

| Aspect | M365 Agents SDK | M365 Agents Toolkit |
|--------|-----------------|---------------------|
| **Type** | Runtime-bibliotheek/framework | IDE-extensie + CLI |
| **Doel** | Agent-logica en hosting | Ontwikkelworkflow-automatisering |
| **Installatie** | Projectafhankelijkheden (NuGet, npm, pip) | VS Code, Visual Studio, npm CLI |
| **Ontwikkelaarinteractie** | Applicatiecode schrijven | Projectcreatie, debugging, deployment |

Zie het zo: de Toolkit **scaffoldt projecten** die **verwijzen naar de SDK**. Wanneer je een nieuwe agent maakt met de Toolkit, genereert het een projectstructuur met de benodigde SDK-pakketten al geconfigureerd.

### Evolutie van Teams Toolkit

De Agents Toolkit is een evolutie van de Teams Toolkit, die Microsoft introduceerde op Build 2020. De rebranding van mei 2025 weerspiegelt Microsoft's strategische verschuiving van het bouwen van Teams-apps naar het bouwen van AI-agents over het gehele Microsoft 365-ecosysteem.

Belangrijk: **Teams Toolkit is niet deprecated**—het is gerebranded. Alle oorspronkelijke mogelijkheden (bots, tabs, message extensions) blijven beschikbaar. Echter, de TeamsFx SDK zal worden deprecated tegen september 2025, met ondersteuning die doorloopt tot september 2026. Microsoft raadt aan om over te stappen naar de Agents SDK voor nieuwe ontwikkeling.

### Beschikbare Templates

De Toolkit biedt templates voor alle drie agent-typen:

**Declarative Agent templates** creëren de JSON-manifeststructuur en ondersteunende bestanden. Deze zijn geschikt wanneer je Copilot wilt uitbreiden zonder aangepaste code.

**Custom Engine Agent templates** bevatten een Echo Agent (minimale baseline) en een Weather Agent die vooraf is geconfigureerd met Semantic Kernel (.NET) of LangChain (JavaScript/Python) verbonden met Azure OpenAI.

**API Plugin templates** helpen je OpenAPI-gebaseerde plugins te maken die declaratieve agents kunnen aanroepen, inclusief opties om te starten vanaf een bestaande OpenAPI-spec of een nieuwe Azure Functions-gebaseerde API te genereren.

### Ontwikkelworkflow

De Toolkit stroomlijnt het ontwikkelproces via verschillende functies:

**Project scaffolding** genereert een complete projectstructuur met juiste configuratie, afhankelijkheden en voorbeeldcode. In plaats van alles vanaf nul op te zetten, krijg je een werkend startpunt.

**Lokale debugging** met de Microsoft 365 Agents Playground maakt snelle iteratie mogelijk zonder te deployen naar Azure. De Playground simuleert de Teams-omgeving lokaal, waardoor je Adaptive Cards, berichtafhandeling en gespreksstromen kunt testen zonder een Microsoft 365-tenant nodig te hebben.

**Dev Tunnels-integratie** stelt je lokale ontwikkelserver bloot aan het internet wanneer je moet testen met de daadwerkelijke Teams-client. De Toolkit configureert automatisch veilige tunneling.

**Infrastructure-as-code** met Bicep-templates definieert je Azure-resources declaratief. De `infra/`-map in je project bevat templates voor App Service, Bot Registration en andere benodigde resources.

**Deployment-automatisering** handelt het bouwen van je project, het aanmaken van Azure-resources en het pushen van je code af. Commando's zoals `atk provision`, `atk deploy` en `atk publish` orchestreren de gehele deployment-pipeline.

### Projectstructuur

Een typisch custom engine agent-project gemaakt door de Toolkit bevat:

```
my-agent/
├── .vscode/              # VS Code-configuratie
│   └── tasks.json        # Debug- en build-taken
├── appPackage/           # Teams/Copilot app-manifest
│   └── manifest.json     # App-definitie met bot-scopes
├── infra/                # Azure Bicep-templates
│   └── azure.bicep       # Infrastructuurdefinitie
├── src/                  # Jouw agent-code
│   ├── Program.cs        # Startpunt en hosting
│   └── MyAgent.cs        # Agent-logica met handlers
├── m365agents.yml        # Lifecycle-configuratie
└── .env.local            # Lokale omgevingsvariabelen
```

### CLI-commando's

De Agents Toolkit CLI (`atk`) biedt commando's voor alle ontwikkelfasen:

`atk new` maakt een nieuw project van templates en doorloopt configuratieopties interactief.

`atk provision --env dev` creëert Azure-resources gedefinieerd in je Bicep-templates.

`atk deploy --env dev` bouwt je project en deployt het naar Azure.

`atk publish --env prod` dient je app-pakket in bij het Teams Admin Center voor organisatiedistributie.

`atk doctor` controleert vereisten en valideert je ontwikkelomgeving.

### Installatie

Voor VS Code, installeer de "Microsoft 365 Agents Toolkit"-extensie vanuit de marketplace.

Voor Visual Studio, voeg de workload toe via Visual Studio Installer onder ASP.NET en webontwikkeling-opties.

Voor command-line gebruik: `npm install -g @microsoft/m365agentstoolkit-cli`

---

## Kennisdeelvraag 5: Hoe Vergelijken Deze Benaderingen?

Nu we elke technologie individueel begrijpen, kunnen we een uitgebreide vergelijking maken om besluitvorming te begeleiden.

### Functievergelijkingsmatrix

| Functie | Declaratieve Agents | Custom Engine Agents | Opmerkingen |
|---------|---------------------|---------------------|-------------|
| **AI-modelselectie** | Alleen Microsoft | Elk model | Custom engine staat Claude, GPT-4, Llama, fijn-afgestemde modellen toe |
| **Orchestratiecontrole** | Geen | Volledig | Custom engine kan Semantic Kernel, LangChain of aangepaste logica gebruiken |
| **Hosting vereist** | Nee | Ja | Declaratief draait op Microsoft-infrastructuur |
| **Ontwikkelcomplexiteit** | Laag | Hoog | Declaratief is configuratie; custom vereist code |
| **Proactieve berichten** | Nee | Ja | Cruciaal voor notificatiescenario's |
| **Multi-channel deployment** | Alleen M365 | 15+ kanalen | Custom engine ondersteunt web, SMS, Slack, etc. |
| **Adaptive Cards** | Beperkt | Volledig | Custom engine ondersteunt Action.Execute, Task Modules |
| **Microsoft Store-publicatie** | Nee | Ja | Vereist voor commerciële distributie |
| **Compliance-overerving** | Automatisch | Handmatig | Declaratief erft M365-compliance automatisch |

### Vergelijking Ontwikkelinspanning

**Declaratieve Agents (uren tot dagen)**:
Het maken van een declaratieve agent kan zo weinig als minuten duren met Agent Builder's no-code interface, of een paar uur met de Agents Toolkit voor complexere configuraties. Geen hosting-infrastructuur nodig—Microsoft handelt alles af.

**Custom Engine Agents via Copilot Studio (dagen tot weken)**:
Dit middenpad gebruikt low-code ontwikkeling met beheerde infrastructuur. Het vereist Power Platform-kennis en basis prompt engineering, maar Microsoft handelt hosting af.

**Custom Engine Agents via M365 Agents SDK (weken tot maanden)**:
Volledige pro-code ontwikkeling die expertise vereist in ASP.NET Core, Node.js of Python. Ontwikkelaars moeten bot-architectuur, AI/ML-concepten, Azure-diensten en gekozen orchestration-frameworks begrijpen. Deployment vereist het provisioneren van Azure-resources, het configureren van Azure Bot Service, het verpakken van app-manifesten en het indienen bij het Microsoft 365 Admin Center.

### Kostenvergelijking

**Declaratieve Agents**: Geen hosting-kosten. Agents die alleen instructies en webzoekopdrachten gebruiken brengen geen extra kosten met zich mee boven de basis Microsoft 365 Copilot-licentie. Agents die SharePoint, Graph-connectors of andere organisatiegegevens raadplegen, verbruiken Copilot Studio metered credits.

**Custom Engine Agents**: De M365 Agents SDK zelf is gratis (MIT-licentie), maar operationele kosten omvatten Azure-hosting (App Service, Container Apps), AI-serviceverbruik (Azure OpenAI-gebruik), Azure Bot Service-registratie en optionele Copilot Studio-licenties voor geïntegreerde functies.

### Beslissingskader

**Kies declaratieve agents wanneer**:
Je workflows opereren binnen Microsoft 365-applicaties, Microsoft's foundation-modellen voldoen aan je AI-vereisten, snelle deployment prioriteit heeft boven customization, ingebouwde compliance-overerving waardevol is, en use cases kennisophaling, documentsamenvatting, IT-ondersteuning of employee self-service omvatten.

**Kies custom engine agents wanneer**:
Domeinspecifieke of fijn-afgestemde AI-modellen vereist zijn, complexe bedrijfslogica volledige controle vereist, externe kanaal-deployment nodig is (klantportals, websites), multi-user groepssamenwerking in Teams vereist is, publicatie naar Microsoft Commercial Store gepland is, of je bestaande Bot Framework-bots migreert.

---

## Kennisdeelvraag 6: Wat is Azure AI Foundry Agent Service?

Azure AI Foundry Agent Service biedt een volledig beheerde runtime voor het bouwen, deployen en opereren van AI-agents op enterprise-schaal—functionerend als "de lijm" die modellen, tools en frameworks verbindt. De architectuur bestaat uit drie kerncomponenten: een **modellaag** (GPT-4o, GPT-4, Llama, etc.), **instructies** die agentgedrag definiëren, en **tools** die kennisophaling en acties mogelijk maken via Bing, SharePoint, Azure AI Search en Azure Logic Apps met meer dan **1.400 connectors**.

### Kernarchitectuur en Ontwikkelworkflow

De ontwikkelworkflow is eenvoudig. Ontwikkelaars creëren een Foundry-project, configureren omgevingsvariabelen voor het project-endpoint en modeldeployment, en instantiëren vervolgens agents met Python, C#, TypeScript of Java SDK's:

```python
from azure.identity.aio import AzureCliCredential
from agent_framework.azure import AzureAIAgentClient

async with (
    AzureCliCredential() as credential,
    AzureAIAgentClient(async_credential=credential).create_agent(
        name="HelperAgent",
        instructions="You are a helpful assistant."
    ) as agent,
):
    result = await agent.run("Hello!")
```

Ingebouwde tools omvatten file search met vector stores, een Python code interpreter sandbox, Bing search voor real-world grounding, en Microsoft Fabric voor data analytics.

### Portabiliteit en Vendor Lock-in: Reële Uitdagingen

De volledig beheerde Foundry-runtime is **alleen Azure**—agentstatus, threads, berichten en bestanden zijn allemaal afhankelijk van Azure-diensten. De basissetup slaat data op in door Microsoft beheerde multi-tenant storage, terwijl standaard setup Azure Cosmos DB, Azure Storage en Azure AI Search vereist voor volledig data-eigenaarschap. Infrastructuurarchitectuur gebruikt de `Microsoft.CognitiveServices/account` resource provider die wordt gedeeld met Azure OpenAI en andere cognitive services.

Echter, het **open-source Microsoft Agent Framework** ondersteunt wel deployment buiten Azure. Microsoft stelt dat "agents overal kunnen draaien, van on-premises tot elke public cloud, met ondersteuning voor container-gebaseerde portabiliteit." Het onderscheid is kritiek: het Agent Framework zelf is draagbaar, maar de volledige Foundry-runtime met enterprise governance, observability en beheerde schaling blijft Azure-gebonden.

Vendor lock-in manifesteert zich via diepe Azure-afhankelijkheden over zes vectoren: storage (Cosmos DB, Azure Storage, AI Search), identiteit (Microsoft Entra ID), networking (VNets, Private Endpoints), observability (Azure Monitor, Application Insights), governance (Azure Policy, Microsoft Defender), en native Microsoft 365-integratie. Industrie-analisten merken op dat "deze omarming van openheid op protocolniveau een krachtige lock-in dynamiek op platformniveau maskeert."

### Eigen Orchestratie-frameworks Meenemen

Semantic Kernel-integratie is native—`AzureAIAgent` is een gespecialiseerd agenttype dat tool calling automatiseert en gespreksgeschiedenis beheert via service-beheerde threads. LangChain en LangGraph worden ondersteund via het `langchain-azure-ai`-pakket, met OpenTelemetry-compatibele tracing die zichtbaar is in Foundry Observability.

**Hosted Agents** (public preview) maken het mogelijk om agents gebouwd met externe frameworks zoals LangGraph of de OpenAI Agents SDK te draaien binnen Foundry's beheerde omgeving, met unified observability over frameworks heen. De praktische richtlijn: architect met open protocollen (MCP, A2A) om portabiliteitsopties te behouden terwijl je Foundry's sterke punten voor enterprise features benut.

### Foundry Agent Service versus M365 Agents SDK

Deze dienen verschillende doeleinden en werken samen in plaats van te concurreren. **Azure AI Foundry Agent Service** is een Platform-as-a-Service voor hosting en orchestratie met ingebouwde multi-agent workflows. De **M365 Agents SDK** handelt publicatie en distributie naar Microsoft productiviteitsapps af—Teams, Outlook, M365 Copilot—maar is geen orchestrator. Microsoft-documentatie stelt expliciet: "Agent 365 SDK is geen agentstack. Het is niet de manier om een agent te creëren of te hosten en komt niet met enige orchestrator of workflow management."

Het **Microsoft Agent Framework** zit onder beide als de open-source runtime die orchestratie biedt via Semantic Kernel en AutoGen-patronen. Alle drie zijn ontworpen voor integratie: Framework biedt bouwblokken, Foundry biedt enterprise hosting, en M365 SDK biedt distributiekanalen.

---

## Kennisdeelvraag 7: Wat zijn de MCP-authenticatie uitdagingen?

MCP-integratie in Copilot Studio volgt een connector-gebaseerde architectuur waarbij MCP-servers worden ontsloten via **Power Platform Custom Connectors** en worden geconsumeerd door agents. De transportlaag ondersteunt momenteel **alleen Streamable HTTP transport**—SSE transport is deprecated met ondersteuning die eindigt in augustus 2025, en STDIO transport (ontworpen voor lokale servers) wordt niet ondersteund.

### Rechten- en Authenticatie-uitdagingen Vermenigvuldigen Over Grenzen

Copilot Studio ondersteunt drie authenticatietypen: none, API key, en OAuth 2.0 (met dynamic discovery via OAuth 2.0 Dynamic Client Registration of handmatige configuratie). De uitdagingen ontstaan omdat MCP-systemen meerdere authenticatie-oppervlakken omvatten—gebruikers authenticeren naar agents, agents naar MCP-servers, en MCP-servers naar upstream services.

Voor enterprise-scenario's beveelt Microsoft **On-Behalf-Of (OBO) authenticatie** aan met Microsoft Entra ID, wat vereist: service app registraties voor de MCP-server, connector app registraties met gedelegeerde rechten, en configuratie van de Azure API Connections service principal (Client ID: `fe053c5f-3692-4f14-aef2-ee34fc081cae`).

OAuth-beveiligde MCP-tools triggeren "additional permissions required" prompts die de gespreksstroom verstoren. Elke gebruiker vereist zijn eigen verbinding naar MCP-servers die auth vereisen, met token refresh en expiration handling geconfigureerd per verbinding.

### Geneste MCP Creëert Onopgeloste Rechten-delegatieproblemen

Wanneer een MCP-server andere MCP-servers aanroept (MCP-in-MCP), ontstaan meerdere kritieke problemen die **niet worden behandeld in huidige Microsoft-documentatie**:

**Token propagation over trust boundaries** wordt problematisch omdat OAuth-tokens zijn gescoped naar enkele resource servers, niet ketens. Het "confused deputy"-probleem ontstaat wanneer agents handelen namens Gebruiker A terwijl ze interageren met Actor B—als context niet correct is gescoped, gaan permission tracking en accountability verloren.

**Consent scope ambiguity** betekent dat wanneer gebruikers toestemming geven om toegang te krijgen tot MCP Server A, die toestemming niet automatisch doorwerkt naar MCP Server B die Server A mogelijk aanroept, of naar upstream API's die Server A's tools mogelijk benaderen. De MCP-specificatie erkent dit, merkend op dat "multi-hop scenario's" een "verwachte evolutie" zijn die preserved on-behalf-of relaties over hops en consistente audit trails vereisen—mogelijkheden die niet bestaan in huidige implementaties.

**Token chaining security** per MCP spec sectie 2.9.4.4 moet propagation delays bij revocation, timing attacks, en het onderhouden van audit trails over de gehele delegatieketen afhandelen. JWTs zijn self-contained, dus het verwijderen van tokens uit databases voorkomt gebruik niet tot expiration.

Noch Microsoft's Copilot Studio-documentatie noch de MCP-specificatie adresseert deze geneste scenario's volledig. Beveiligingsonderzoekers waarschuwen voor "Tool Chaining Leaks" waarbij agents die werken met meerdere MCP-servers per ongeluk credentials tussen systemen transfereren.

### Copilot Studio MCP Connector Beperkingen

Verschillende technische beperkingen beïnvloeden implementatie:

- **Schema-beperkingen**: `System.FormatException` wordt gegooid wanneer `exclusiveMinimum` integer is in plaats van Boolean; array-type tool definities worden afgekapt; **reference type inputs/outputs worden niet ondersteund** (tools die deze gebruiken worden uitgefilterd)
- **Custom header beperkingen**: Headers die het `X-` prefix patroon gebruiken werken niet met MCP connectors; geen UI om custom headers toe te voegen—vereist directe OpenAPI specificatie bewerking
- **Feature support**: Alleen MCP tools en resources worden ondersteund; **prompts** (voorgedefinieerde templates) worden nog niet volledig ondersteund; generative orchestration moet zijn ingeschakeld als voorwaarde
- **Governance gaps**: IP-restricties in Azure Firewall staan alle Power Platform-omgevingen toe ongeacht tenant—MCP deployments moeten nog steeds worden behandeld als publiek toegankelijk

---

## Kennisdeelvraag 8: Wat zijn de geavanceerde M365 Agents SDK-patronen?

De M365 Agents SDK evolueerde vanuit Bot Framework en ondersteunt C#, JavaScript en Python met deployment naar Teams, M365 Copilot, webchat en 10+ third-party kanalen. Deze sectie behandelt geavanceerde interactiepatronen die essentieel zijn voor enterprise-implementaties.

### Slash Commands Vereisen Handmatige Implementatie

De SDK heeft geen dedicated slash command feature als first-class concept. Commands worden geïmplementeerd via activity-based pattern matching:

```csharp
agent.OnActivity(ActivityTypes.Message, async (turnContext, turnState, cancellationToken) => {
    var text = turnContext.Activity.Text?.Trim().ToLower();
    switch (text) {
        case "/help":
            await turnContext.SendActivityAsync("Beschikbare commando's: /help, /status, /settings");
            break;
        case "/status":
            await turnContext.SendActivityAsync("Systeem operationeel.");
            break;
        case "/settings":
            await SendSettingsCard(turnContext, cancellationToken);
            break;
    }
});
```

Teams ondersteunt wel native slash commands waarbij gebruikers `/` typen in de compose box, maar dit is platformfunctionaliteit in plaats van SDK-provided.

### Proactieve Berichten Vereisen Opgeslagen Gespreksreferenties

Proactieve berichten gebruiken opgeslagen conversation references met `continueConversation`:

```csharp
// Sla referentie op tijdens initiële interactie
var conversationReference = turnContext.Activity.GetConversationReference();

// Later, stuur proactief bericht
await adapter.ContinueConversationAsync(
    botAppId,
    conversationReference,
    async (context, token) => {
        await context.SendActivityAsync("Proactieve notificatie!");
    },
    cancellationToken);
```

Proactieve mogelijkheden omvatten Teams-notificaties (1:1, groep, kanalen), activity feed notificaties, asynchrone berichten (vereist **15-seconden initiële respons**, **45-seconden timeout** tussen streaming updates), en email via Microsoft Graph-integratie. Apps kunnen proactief worden geïnstalleerd met Graph API om conversation ID's te verkrijgen.

### Suggested Actions en Adaptive Cards Hebben Specifieke Beperkingen

Teams ondersteunt suggested actions met **alleen het `imBack` action type** en toont **maximaal 6 suggested actions**:

```csharp
reply.SuggestedActions = new SuggestedActions() {
    Actions = new List<CardAction>() {
        new CardAction() { Title = "Optie 1", Type = ActionTypes.ImBack, Value = "optie1" },
        new CardAction() { Title = "Optie 2", Type = ActionTypes.ImBack, Value = "optie2" }
    }
};
```

Adaptive Card support omvat `Action.Submit`, `Action.Execute` (universal actions voor Teams/Outlook), `Action.OpenUrl`, `Action.ShowCard`, `Action.ToggleVisibility`, en `Action.ResetInputs`. Input controls dekken text, number, date, time, toggle, choice sets, people pickers, en typeahead search. Schema versie 1.6 wordt ondersteund in Web Chat, versie 1.5 in Dynamics 365 Omnichannel.

### Bestandsafhandeling, Teams Features en Rate Limits

Bestandsuploads vereisen `"supportsFiles": true` in het manifest. Twee methoden bestaan: Microsoft Graph API's werken voor alle scopes maar vereisen OAuth, terwijl Teams Bot API's beperkt zijn tot alleen personal context. Het file consent pattern omvat het versturen van een `FileConsentCard`, het ontvangen van gebruikersacceptatie via `fileConsent/invoke`, dan het uitvoeren van HTTP POST naar de verstrekte upload URL.

Teams-specifieke features omvatten @mentions (het toevoegen van `Mention` entities aan activities), tag mentions voor kanalen, en **Resource-Specific Consent (RSC)** met `ChannelMessage.Read.Group` permission om alle kanaalberichten te ontvangen zonder @mention. Typindicatoren tonen gedurende ongeveer 3 seconden maar **tonen niet in channel threaded conversations**.

Rate limits zijn significante beperkingen: **50 RPS per app per tenant** globaal, met per-thread limits van 7 berichten per seconde (8 per 2 seconden, 60 per 30 seconden, 1800 per uur). Berichtgrootte limits zijn **28 KB voor incoming webhooks** en **100 KB voor bot berichten**. Applicaties moeten exponential backoff retry strategieën implementeren.

### Long-Running Operations, Agenda-integratie en Staatsbeheer

Long-running operations vereisen de 15-seconden initiële respons regel, met informatieve updates verstuurd binnen 45-seconden windows. Streaming responses gebruiken `StreamingActivityHandler` voor real-time updates met Azure AI-integratie.

Agenda-events en Teams meeting invites vereisen **Microsoft Graph API-integratie**—er is geen directe SDK-ondersteuning. De Agent 365 SDK kan Microsoft MCP-servers gebruiken voor email, SharePoint-documenten en agenda-operaties.

Multi-turn dialogs gebruiken een drie-laags staatsbeheer systeem: storage laag (MemoryStorage voor development, Azure Blob of Cosmos DB voor productie), state buckets (UserState, ConversationState), en AgentApplication met automatische state loading/saving. Voor complexe conversational flows biedt het `@microsoft/agents-hosting-dialogs`-pakket gestructureerd dialog management.

### Belangrijke SDK-beperkingen

Verschillende mogelijkheden hebben gaps: native slash commands vereisen handmatige implementatie, typindicatoren werken niet in channel threads, file uploads via Teams API's zijn beperkt tot personal context, read receipts worden niet direct ondersteund, agenda-integratie vereist Graph API, human handoff vereist third-party integratie (Omnichannel, LivePerson) of custom implementatie, en posten naar private channels wordt niet ondersteund.

---

## Deel 2: Druppie Integratie-aanbeveling

Met de fundamentele kennis vastgelegd, kunnen we nu onze primaire onderzoeksvraag adresseren: Hoe moet Druppie integreren met Microsoft 365 Copilot?

### Waarom Custom Engine Agents Vereist Zijn voor Druppie

De analyse is duidelijk: **Druppie vereist custom engine agents** vanwege verschillende fundamentele vereisten die declaratieve agents niet kunnen vervullen:

**Multi-agent orchestratie**: Druppie's architectuur bevat meerdere gespecialiseerde sub-agents (SPEC Agent, Code Agent, Review Agent, Security Agent) gecoördineerd door een Router Agent. Declaratieve agents bieden geen mechanisme voor dit soort orchestratie.

**Aangepaste AI-integratie**: Druppie gebruikt OpenCode voor orchestratie, wat mogelijk niet-Microsoft AI-modellen betreft. Declaratieve agents ondersteunen alleen Microsoft's foundation-modellen.

**On-premises connectiviteit**: Core Druppie draait on-premises en maakt verbinding met interne systemen (Gitea, DataLab). Hoewel declaratieve agents Graph-connectors kunnen gebruiken, kunnen ze de aangepaste authenticatie en connectiviteit die Druppie vereist niet accommoderen.

**Per-aanroep toestemmingsstromen**: Druppie's beveiligingsmodel vereist expliciete gebruikerstoestemming voordat schrijfoperaties worden uitgevoerd. Dit vereist aangepaste Adaptive Card-stromen die declaratieve agents niet ondersteunen.

**Multi-channel toegang**: Druppie moet toegankelijk zijn via Copilot, Teams en een dedicated webportaal. Deze multi-channel deployment vereist custom engine-mogelijkheden.

### Aanbevolen Architectuur: Thin Client-patroon

De aanbevolen architectuur houdt het OpenCode-gebaseerde custom multi-agent systeem draagbaar terwijl Microsoft-frontends worden gebruikt:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Microsoft 365 Frontend                        │
│  (Teams Chat, Copilot Chat, SharePoint, Web)                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Activities
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│              Azure Bot Service (Alleen Middleware)               │
│  - Kanaalroutering                                               │
│  - Tokenservice                                                  │
│  - OAuth-verbindingsbeheer                                       │
└─────────────────────────┬───────────────────────────────────────┘
                          │ HTTPS POST naar /api/messages
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│           Thin Proxy Agent (M365 Agents SDK)                     │
│  - Handelt Copilot/Teams activity protocol af                    │
│  - Beheert authenticatie/SSO                                     │
│  (Kan overal draaien: Azure, AWS, GCP, on-prem)                  │
└─────────────────────────┬───────────────────────────────────────┘
                          │ REST/gRPC
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│              Druppie Core (Self-Hosted, Draagbaar)               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   Router Agent (OpenCode)                 │   │
│  └───────────┬────────────┬────────────┬────────────────────┘   │
│              │            │            │                         │
│       ┌──────▼──┐  ┌──────▼──┐  ┌──────▼──┐  ┌──────────┐       │
│       │  SPEC   │  │  Code   │  │ Review  │  │ Security │       │
│       │  Agent  │  │  Agent  │  │  Agent  │  │  Agent   │       │
│       └─────────┘  └─────────┘  └─────────┘  └──────────┘       │
│                                                                  │
│  MCP Server (jouw tools) ◄──── MCP Client (Foundry of custom)   │
└─────────────────────────────────────────────────────────────────┘
```

### Componentverantwoordelijkheden

| Component | Locatie | Verantwoordelijkheid |
|-----------|---------|----------------------|
| **Azure Bot Service** | Microsoft Azure (beheerd) | Routeert M365-berichten, tokenbeheer |
| **Thin Client** | Azure (jouw code) | Protocolvertaling, tokenextractie |
| **Core Druppie** | On-premises | Bedrijfslogica, OpenCode-orchestratie, tools, auth |
| **Portal** | On-premises | Directe webtoegang, omzeilt M365-beperkingen |

Deze architectuur biedt verschillende voordelen. De Thin Client handelt Microsoft-specifieke protocollen af (Activity Protocol, OAuth-stromen, Adaptive Cards) terwijl Core Druppie volledig draagbaar blijft. Als je later Druppie wilt deployen naar een ander chatplatform, hoef je alleen een nieuwe thin client te maken—de kernbedrijfslogica blijft ongewijzigd.

### Vijf Integratiepaden Vergeleken

| Benadering | Aangepaste Orchestratie | Inline Toestemming | Hosting Flexibiliteit | Complexiteit |
|------------|-------------------------|--------------------|-----------------------|--------------|
| **Copilot Studio** | Beperkt | ✅ Authenticate node | Alleen SaaS | Low-code |
| **M365 Agents SDK + Custom Engine** | ✅ Volledig | ✅ OAuthPrompt, Adaptive Cards, Task Modules | Self-hosted overal | Pro-code |
| **Azure AI Foundry Agent Service** | ✅ Volledig | ✅ OAuth 2.0 + MCP auth | Azure-centrisch met hybride | Medium |
| **Declaratieve Agents** | Geen | Gedeeltelijk (alleen SSO) | Geen hosting vereist | No-code |

De **M365 Agents SDK + Custom Engine**-benadering biedt de optimale balans voor Druppie: volledige orchestratiecontrole, maximale authenticatieflexibiliteit en de mogelijkheid om overal te hosten terwijl naadloos wordt geïntegreerd met Microsoft 365.

---

## Adaptive Cards: Multi-Platform UI voor Approval Flows

Microsoft Adaptive Cards zijn JSON-gebaseerde UI-kaarten die uniform werken over Teams, Outlook en Copilot. Voor Druppie's multi-channel architectuur is het cruciaal te begrijpen welke features op welk platform werken. **De belangrijkste conclusie: Action.Execute is de universele actie-type die overal werkt en ideaal is voor approval flows.**

### Platformvergelijking

| Feature | Teams Desktop/Web | Teams Mobile | Copilot Chat | Outlook |
|---------|-------------------|--------------|--------------|---------|
| **Schema versie** | v1.5 | v1.2 | v1.5 | v1.4+ (Universal Actions) |
| **Action.Submit** | ✅ (alleen bots) | ✅ | ✅ | ❌ |
| **Action.Execute** | ✅ | ✅ | ✅ | ✅ |
| **Action.OpenUrl** | ✅ | ✅ | ❌ | ✅ |
| **Action.ToggleVisibility** | ✅ | ✅ | ❌ | ✅ |
| **Refresh/User Views** | ✅ (max 60 users) | ✅ | Beperkt | ✅ |
| **Task Modules** | ✅ | ✅ | ❌ | ❌ |

### Action.Execute: De Cross-Platform Oplossing

Voor Druppie's toestemmingsflows is **Action.Execute** (Universal Actions, v1.4+) de aangewezen keuze:

- **Cross-platform compatibiliteit**: Werkt in Teams, Outlook én Copilot Chat
- **Bot-gebaseerde afhandeling**: De bot ontvangt een `adaptiveCard/action` invoke activity
- **Kaart-updates mogelijk**: Ondersteunt inline refresh van de kaart na gebruikersactie
- **Fallback-ondersteuning**: Voeg `fallback: "Action.Submit"` toe voor oudere clients

De M365 Agents SDK handelt Action.Execute responses automatisch af via de CloudAdapter en TurnContext, wat naadloos integreert met de aanbevolen Thin Client-architectuur.

### Praktische Richtlijnen

**Versiekeuze**: Gebruik schema v1.5 als basis, maar test op Teams mobile (v1.2). Zet de `version` property correct en gebruik `fallback` voor nieuwe features.

**Layout**: Gebruik single-column layouts voor maximale compatibiliteit. Vermijd fixed widths; gebruik `auto` of `stretch`. Houd berichten onder **28KB** voor alle messaging channels.

**Outlook-specifiek**: Vereist `originator` registratie in het Actionable Email Developer Dashboard. Zonder registratie worden kaarten niet getoond.

### Wat Dit Betekent voor Druppie

Met de M365 Agents SDK en Action.Execute kunnen Druppie's approval flows consistent werken over alle Microsoft-kanalen. De Thin Client verstuurt Adaptive Cards via `MessageFactory.Attachment()`, en dezelfde kaart met dezelfde approval-knoppen functioneert in Teams, Copilot Chat én Outlook—waardoor gebruikers in hun voorkeursomgeving kunnen werken zonder functieverlies voor de kernworkflow.

---

## Authenticatiearchitectuur

### Identiteitsstroom

```
Gebruiker (M365) 
    → Entra ID (authenticeert)
    → Azure Bot Service (routeert)
    → Thin Client (extraheert token)
    → Keycloak (federeert identiteit, past RBAC toe)
    → Core Druppie (voert uit met gebruikerscontext)
    → MCP Tools (gebruiken geschikte tokens)
```

De Thin Client ontvangt het Entra ID-token van de gebruiker via Microsoft's SSO-mechanisme. Het federeert deze identiteit vervolgens naar Keycloak, dat role-based access control (RBAC) onderhoudt voor Druppie-specifieke rechten. Deze scheiding zorgt ervoor dat Microsoft authenticatie afhandelt terwijl Druppie autorisatiecontrole behoudt.

### Tokengebruik per Service

| Service | Tokenbron | Stroom |
|---------|-----------|--------|
| DataLab | Entra ID | Forward gebruikers Entra-token direct |
| Gitea | Keycloak | Gebruik Keycloak-token (gefedereerd vanuit Entra) |
| Microsoft Graph | Entra ID | OBO-stroom voor gedelegeerde rechten |

### On-Behalf-Of (OBO) Stroom

Voor toegang tot Graph API (Teams, Mail, Calendar) wisselt de Thin Client het token van de gebruiker uit voor een met de juiste scopes:

De gebruiker authenticeert bij M365 en ontvangt een Entra-token. De Thin Client ontvangt dit token via SSO. De Thin Client roept `ExchangeTurnTokenAsync` aan met Graph-scopes. Entra ID retourneert een gedelegeerd Graph-token. Core Druppie gebruikt dit token voor Graph API-aanroepen namens de gebruiker.

---

## Per-Tool Rechten en Toestemming

### Rechtenmodel

Druppie implementeert een granulair rechtenmodel waarbij leesoperaties SSO-tokens gebruiken, maar schrijfoperaties expliciete per-aanroep toestemming vereisen:

| Tool | Operatie | Toestemming Vereist | Reden |
|------|----------|---------------------|-------|
| **DataLab** | SELECT/Query | ❌ SSO voldoende | Alleen-lezen toegang |
| **DataLab** | INSERT/UPDATE | ✅ Per-aanroep | Wijzigt data |
| **DataLab** | DELETE | ✅ Per-aanroep | Destructieve actie |
| **Gitea** | Read/Clone | ❌ SSO voldoende | Eigen repo lezen |
| **Gitea** | Commit | ✅ Per-aanroep | Wijzigt codebase |
| **Gitea** | Create PR | ✅ Per-aanroep | Workflow-actie |
| **Gitea** | Merge PR | ✅ Per-aanroep | Beïnvloedt main branch |
| **Gitea** | Delete branch | ✅ Per-aanroep | Destructieve actie |
| **Teams** | Read messages | ❌ SSO voldoende | Eigen gesprekken lezen |
| **Teams** | Send message | ✅ Per-aanroep | Verzendt ALS de gebruiker |
| **Mail** | Read inbox | ❌ SSO voldoende | Eigen mail lezen |
| **Mail** | Send email | ✅ Per-aanroep | Verzendt ALS de gebruiker |
| **Calendar** | Read events | ❌ SSO voldoende | Eigen agenda lezen |
| **Calendar** | Create event | ✅ Per-aanroep | Wijzigt agenda |

### Voorbeeld Toestemmingsstroom

Hier is hoe een typische toestemmingsstroom werkt wanneer een gebruiker Druppie vraagt om een Teams-bericht te versturen:

1. **Gebruikersverzoek**: "@Druppie notificeer engineering dat de build klaar is"
2. **Intent parsing**: Druppie bepaalt dat dit de Teams Send-tool vereist
3. **RBAC-check**: Keycloak verifieert dat de gebruiker de `teams-send`-rol heeft
4. **Toestemmingsverzoek**: Core Druppie retourneert een toestemmingsverzoek naar de Thin Client
5. **UI-presentatie**: De Thin Client toont een Adaptive Card met de voorgestelde actie:

```
┌─────────────────────────────┐
│ ⚠️ Bevestig Actie            │
│                             │
│ Bericht versturen naar      │
│ #engineering?               │
│                             │
│ "De build is klaar..."      │
│                             │
│ [Goedkeuren]    [Weigeren]  │
└─────────────────────────────┘
```

6. **Gebruikersgoedkeuring**: Gebruiker klikt Goedkeuren
7. **Uitvoering**: Core Druppie roept de Teams MCP-tool aan, die Graph API aanroept
8. **Bevestiging**: UI toont "✓ Bericht verstuurd naar #engineering"

Dit toestemmingsmodel zorgt ervoor dat gebruikers controle behouden over acties die namens hen worden uitgevoerd terwijl efficiënte automatisering mogelijk wordt wanneer gepast.

---

## Hybride Connectiviteitsopties
**(dit is puur ai-advies: meer onderzoek nodig + overleg netwerk/microsoft specialist nodig)**
Als Core Druppie on-premises draait en bereikt moet worden vanuit Azure, zijn er verschillende connectiviteitsopties beschikbaar:

| Optie | Beveiliging | Complexiteit | Maandelijkse Kosten | Use Case |
|-------|-------------|--------------|---------------------|----------|
| **Azure Relay** | ⭐⭐⭐⭐ | Laag | €40-50 | API-niveau toegang |
| **VPN Gateway** | ⭐⭐⭐⭐ | Medium | €140-2.665 | Volledig netwerk |
| **ExpressRoute** | ⭐⭐⭐⭐⭐ | Hoog | €1.000-8.500+ | Mission-critical |
| **APIM Self-Hosted** | ⭐⭐⭐⭐ | Medium-Hoog | €50-2.800 | API-management |

### Aanbevolen: Azure Relay Hybrid Connections

Azure Relay biedt het eenvoudigste pad voor Druppie's vereisten. Het vereist geen inbound firewall-poorten (alleen outbound 443), gebruikt de Hybrid Connection Manager die on-premises wordt geïnstalleerd, biedt TLS 1.2-encryptie, en kost ongeveer €10/listener plus €1/GB overage.

Deze aanpak stelt de Thin Client in Azure in staat om te communiceren met Core Druppie on-premises zonder inbound firewall-poorten te openen of VPN-tunnels op te zetten.

---

## Recente Microsoft-aankondigingen

Het agent-ecosysteem evolueert snel. Belangrijke ontwikkelingen van 2024-2025:

**Build 2024** introduceerde declaratieve agents (toen "declarative copilots" genoemd) en custom engine agents als het tweepaden-uitbreidbaarheidsmodel. De Teams Toolkit kreeg ondersteuning voor custom engine copilot.

**Oktober 2024** markeerde algemene beschikbaarheid voor declaratieve agents en API-plugins.

**Ignite 2024** kondigde de M365 Agents SDK aan en introduceerde Agent 365, een control plane voor het beheren en beveiligen van agents over frameworks heen. Microsoft onthulde ook Entra Agent ID voor automatische identiteitstoewijzing en governance.

**Build 2025** bracht significante verbeteringen: Model Context Protocol (MCP)-ondersteuning voor eenvoudigere tool-integratie, multi-agent orchestratie in Copilot Studio, declaratieve agents in Word en PowerPoint (naast Teams en Copilot Chat), en Microsoft 365 Copilot Tuning voor low-code modelaanpassing. De Teams Toolkit werd officieel gerebranded naar M365 Agents Toolkit.

**Ignite 2025** kondigde het Microsoft Agent Framework aan, dat AutoGen en Semantic Kernel samenvoegt in een unified framework dat runtime zal delen met de M365 Agents SDK. De Agent 365 SDK werd geïntroduceerd voor enterprise governance-, beveiligings- en compliance-functies.

---

## Belangrijke Documentatiereferenties

### Kernintegratie

| Onderwerp | URL |
|-----------|-----|
| Agents Overzicht | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/agents-overview |
| Custom Engine Agents | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/overview-custom-engine-agent |
| Beslissingsgids | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/decision-guide |
| Activity Protocol | https://learn.microsoft.com/en-us/microsoft-365/agents-sdk/activity-protocol |
| Bekende Problemen | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/known-issues |

### M365 Agents SDK

| Onderwerp | URL |
|-----------|-----|
| SDK Overzicht | https://learn.microsoft.com/en-us/microsoft-365/agents-sdk/agents-sdk-overview |
| Hoe Agents Werken | https://learn.microsoft.com/en-us/microsoft-365/agents-sdk/how-agent-works-sdk |
| OAuth Configuratie | https://learn.microsoft.com/en-us/microsoft-365/agents-sdk/agent-oauth-configuration-dotnet |
| Staatsbeheer | https://learn.microsoft.com/en-us/microsoft-365/agents-sdk/state-concepts |
| Proactieve Berichten | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/custom-engine-agent-asynchronous-flow |
| GitHub (.NET) | https://github.com/Microsoft/Agents-for-net |
| GitHub (JS) | https://github.com/Microsoft/Agents-for-js |
| GitHub (Python) | https://github.com/Microsoft/Agents-for-python |

### M365 Agents Toolkit

| Onderwerp | URL |
|-----------|-----|
| Toolkit Overzicht | https://learn.microsoft.com/en-us/microsoft-365/developer/overview-m365-agents-toolkit |
| Toolkit Fundamentals | https://learn.microsoft.com/en-us/microsoftteams/platform/toolkit/agents-toolkit-fundamentals |
| CLI Referentie | https://learn.microsoft.com/en-us/microsoftteams/platform/toolkit/microsoft-365-agents-toolkit-cli |
| Agents Playground | https://learn.microsoft.com/en-us/microsoftteams/platform/toolkit/debug-your-agents-playground |
| GitHub | https://github.com/OfficeDev/microsoft-365-agents-toolkit |

### Authenticatie

| Onderwerp | URL |
|-----------|-----|
| API Plugin Auth | https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/api-plugin-authentication |
| OBO Stroom | https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow |
| Agent OBO Stroom | https://learn.microsoft.com/en-us/entra/agent-id/identity-platform/agent-on-behalf-of-oauth-flow |

### MCP Protocol

| Onderwerp | URL |
|-----------|-----|
| MCP Specificatie | https://modelcontextprotocol.io/ |
| MCP Autorisatie | https://modelcontextprotocol.io/docs/tutorials/security/authorization |
| Copilot Studio MCP | https://learn.microsoft.com/en-us/microsoft-copilot-studio/agent-extend-action-mcp |
| Foundry MCP | https://learn.microsoft.com/en-us/azure/ai-foundry/agents/how-to/tools/model-context-protocol |

---

## Conclusie

### Drie Kritieke Architecturale Inzichten

Drie architecturale inzichten komen naar voren uit de technische deep-dive analyse:

**Portabiliteitsclaims vereisen grondige beoordeling**: Hoewel het Microsoft Agent Framework daadwerkelijk draagbaar is, blijft de volledige Azure AI Foundry beheerde runtime met enterprise governance alleen Azure, wat betekenisvolle lock-in creëert ondanks ondersteuning voor open protocollen zoals MCP en A2A.

**M365 Agents SDK is een distributiemechanisme, geen orchestratielaag**: Ontwikkelaars moeten het combineren met Foundry of een andere hostingoplossing om complete agentsystemen te bouwen. De SDK handelt publicatie naar Microsoft 365-kanalen af, maar biedt geen agent-orchestratie of workflow management.

### Aanbeveling voor Druppie

De M365 Agents SDK + Custom Engine Agent-benadering biedt de optimale balans voor het integreren van Druppie met Microsoft 365 Copilot. Deze aanbeveling is gebaseerd op vijf belangrijke factoren:

**Volledige orchestratiecontrole**: Het OpenCode multi-agent systeem met gespecialiseerde sub-agents blijft draagbaar en kan onafhankelijk van Microsoft's platformwijzigingen evolueren.

**MIT-gelicenseerde SDK**: Geen vendor lock-in voor de integratielaag. De SDK is open source, en het Thin Client-patroon betekent minimale Microsoft-specifieke code.

**Meerdere toegangspunten**: Dezelfde backend bedient Copilot, Teams en het Druppie Portal, waardoor gebruikers keuze hebben gebaseerd op hun workflowvoorkeuren.

**Secure by design**: Entra ID handelt authenticatie af terwijl Keycloak-federatie Druppie-specifieke RBAC mogelijk maakt. Per-aanroep toestemming zorgt ervoor dat gebruikers controle behouden over schrijfoperaties.

**Productieklaar**: Duidelijk implementatiepad met beheersbare kosten en goed gedocumenteerde API's.

De thin client-architectuur zorgt ervoor dat Core Druppie draagbaar en on-premises blijft terwijl naadloze toegang wordt geboden via de M365 Copilot-interface die gebruikers al kennen. Deze aanpak positioneert Druppie voor langetermijnflexibiliteit terwijl Microsoft's agent-ecosysteem blijft evolueren.

---

