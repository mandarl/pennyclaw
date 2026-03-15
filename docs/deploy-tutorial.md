# Deploy PennyClaw to GCP Free Tier

## Welcome!

This tutorial will deploy PennyClaw — your $0/month personal AI agent — to GCP's Always Free e2-micro instance.

**What you'll get:**
- A 24/7 AI assistant running on your own server
- Web-based chat interface
- Persistent conversation memory
- Tool execution (shell, files, web search)
- All for $0/month, forever

**Time required:** ~5 minutes

Click **Start** to begin.

## Pre-Flight Checks

First, let's make sure your GCP account is ready. The deployment script will automatically check:

- ✓ Authentication status
- ✓ Billing configuration
- ✓ Existing VM instances (free tier allows only 1 e2-micro)
- ✓ Region eligibility
- ✓ Disk and network configuration

Run the pre-flight checks:

```bash
bash scripts/deploy.sh --preflight-only
```

If all checks pass, proceed to the next step.

## Set Your API Key

PennyClaw needs an LLM API key to function. Set your preferred provider:

**OpenAI:**
```bash
export OPENAI_API_KEY="your-key-here"
```

**Anthropic:**
```bash
export ANTHROPIC_API_KEY="your-key-here"
```

You can get an API key from:
- OpenAI: https://platform.openai.com/api-keys
- Anthropic: https://console.anthropic.com/

## Deploy

Now deploy PennyClaw with a single command:

```bash
bash scripts/deploy.sh
```

The script will:
1. Create an e2-micro VM in the best free-tier region
2. Configure a 512MB swap file for extra memory
3. Install PennyClaw as a systemd service
4. Set up automatic security updates
5. Configure the firewall for web access
6. Run health checks to verify everything works

## Access Your Agent

Once deployment is complete, the script will display your access URL:

```
http://YOUR_IP:3000
```

Open this URL in your browser to start chatting with your AI agent!

## Next Steps

- **Secure access:** Set up a Cloudflare Tunnel for HTTPS
- **Add channels:** Enable Telegram or Discord bots
- **Customize:** Edit the system prompt and skills
- **Monitor:** Check logs with `make logs`

## Teardown

To remove PennyClaw and all associated resources:

```bash
bash scripts/teardown.sh
```

This will delete the VM instance and firewall rules. No further charges will be incurred.
