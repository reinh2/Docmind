# DocMind

[![CI](https://github.com/reinh2/docmind/actions/workflows/ci.yml/badge.svg)](https://github.com/reinh2/docmind/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**DocMind is an AI assistant that answers customer questions using only a company's own documents — and always shows exactly where each answer came from.**

🇷🇺 [Читать на русском](README.ru.md) · 🛠️ [Technical documentation for engineers](docs/ENGINEERING.md)

## Video overview

[![DocMind video overview: answers from documents with their sources](https://reinh2.github.io/assets/motion/docmind-en-poster.webp)](https://reinh2.github.io/assets/motion/docmind-en.mp4)

▶ **[Watch the 28-second overview](https://reinh2.github.io/assets/motion/docmind-en.mp4)** (with sound) · [Russian version](https://reinh2.github.io/assets/motion/docmind-ru.mp4) · [Portfolio](https://reinh2.github.io/en/#docmind)

## The problem

Most companies already have the real answers written down somewhere — price lists, FAQs, policies, handbooks. Customers still end up emailing, calling, or waiting in a chat queue to get something that was already sitting in a document. Off-the-shelf chatbots try to "solve" this by guessing at an answer when they're not sure — and a chatbot that confidently invents a wrong price or a wrong policy is often worse than no chatbot at all.

## What DocMind does differently

You give DocMind your documents. It reads them, and from then on it will only ever answer using what's actually written in those documents — nothing invented, nothing assumed.

- **It never makes things up.** If the answer isn't in your documents, it says so honestly and offers to connect the customer with a person, instead of inventing something that merely sounds right.
- **Every answer comes with its receipts.** Customers — and your team — can see exactly which document and which paragraph an answer was based on.
- **It works where your customers already are.** Drop it into your website as a chat bubble, or run it as a Telegram bot. Same brain, same documents, same rules, either way.
- **Your team gets a control room.** A dashboard shows what customers are actually asking, how often the assistant had to say "I don't know," and lets your team review any conversation end-to-end — including the exact evidence the assistant used to answer.

## What it looks like

| Overview dashboard | Managing your documents |
|---|---|
| [![Admin overview](docs/admin-overview.png)](docs/admin-overview.png) | [![Knowledge management](docs/admin-knowledge.png)](docs/admin-knowledge.png) |

| Reviewing a conversation and its evidence | The assistant on a customer's website |
|---|---|
| [![Conversations](docs/admin-conversations.png)](docs/admin-conversations.png) | [![Widget](docs/widget.png)](docs/widget.png) |

## Why the sources matter

Imagine a small law firm's assistant is asked "How much does an initial consultation cost?" A typical chatbot might blend a half-remembered number from its training data with something plausible-sounding and answer confidently — and be wrong. DocMind instead looks through the firm's actual price list, finds the real answer, and shows the exact paragraph it came from. If a customer asks something the firm has never written down anywhere, DocMind says so plainly instead of guessing, and offers to bring in a person. That difference — refusing to guess, and showing your work — is what makes an assistant like this safe to put in front of real customers.

## About this project

DocMind is a demonstration project. The knowledge base in the demo is a set of fictional documents from a made-up Berlin law firm, but the underlying system works the same way for any company's documents. It was built to show what a trustworthy, production-minded AI assistant looks like — not just a chatbot demo, but a considered answer to "can we actually trust this?" The repository is prepared as a portfolio piece.

If you're an engineer evaluating the code, the architecture, or want to run it yourself, the **[Engineering Guide](docs/ENGINEERING.md)** has everything: setup instructions, architecture diagrams, the API reference, and configuration options.

## License

Released under the [MIT License](LICENSE).
