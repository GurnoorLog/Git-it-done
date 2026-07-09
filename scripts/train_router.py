import json, re, string, math, random
from collections import Counter

random.seed(42)

CATEGORIES = [
    "code_generation", "code_debugging", "math", "logical",
    "sentiment", "ner", "summarization", "factual", "extraction"
]

def tokenize(text):
    text = text.lower()
    text = re.sub(r'[^\w\s]', ' ', text)
    return [w for w in text.split() if len(w) > 1 and not w.isdigit()]

# --- Training data ---
def gen_code_gen():
    tasks = []
    templates = [
        "Write a {}",
        "Create a {}",
        "Implement a {} in Python",
        "Build a function to {}",
        "Write code for {}",
        "Generate a {} class",
        "Write a program that {}",
        "Code a function to {}",
        "Implement an algorithm to {}",
        "Write a script to {}",
    ]
    subjects = [
        ("function to sort a list", "Sort a list of integers"),
        ("class for a binary search tree", "Build a binary search tree"),
        ("function to find all prime numbers", "Find all prime numbers less than N"),
        ("function to reverse a linked list", "Reverse a linked list"),
        ("program that implements a queue", "Implement a queue using two stacks"),
        ("function to calculate Fibonacci", "Calculate the nth Fibonacci number"),
        ("class for a simple cache", "Implement an LRU cache"),
        ("function to merge two sorted arrays", "Merge two sorted arrays"),
        ("function to validate a binary search tree", "Validate if a tree is BST"),
        ("function that checks for balanced parentheses", "Check for balanced parentheses"),
    ]
    for s, _ in subjects:
        for t in templates[:3]:
            tasks.append(t.format(s))
    for _, s in subjects:
        tasks.append(s)
    return [(t, "code_generation") for t in tasks]

def gen_code_debug():
    tasks = [
        ("Fix the bug in this code: def add(a, b): return a - b", "code_debugging"),
        ("Debug the following function: for i in range(len(arr)): arr[i] = arr[i] * 2", "code_debugging"),
        ("Fix the sorting algorithm, it doesn't sort correctly", "code_debugging"),
        ("This code has an off-by-one error, please fix it", "code_debugging"),
        ("The function returns None when it should return a value", "code_debugging"),
        ("Debug: my program crashes with IndexError", "code_debugging"),
        ("Fix the bug in this implementation of binary search", "code_debugging"),
        ("This code has a memory leak, can you fix it?", "code_debugging"),
        ("The comparison logic is wrong, correct it", "code_debugging"),
        ("Find and fix the bug in this merge sort implementation", "code_debugging"),
        ("Fix the issue with this code that causes infinite loop", "code_debugging"),
        ("Debug this Python function that calculates averages", "code_debugging"),
        ("What is wrong with this code? It should compute GCD", "code_debugging"),
        ("Fix the race condition in this multithreaded code", "code_debugging"),
    ]
    return tasks

def gen_math():
    tasks = []
    templates = [
        "What is {}?",
        "Solve: {}",
        "Calculate {}",
        "Find {}",
        "Compute {}",
        "How much is {}?",
        "Solve for x: {}",
    ]
    fillers = [
        "15 plus 27",
        "48 divided by 6",
        "the area of a rectangle with length 5 and width 3",
        "square root of 144",
        "15 percent of 200",
        "the sum of 23, 45, and 67",
        "a triangle with base 10 and height 8",
        "2x plus 5 equals 15",
        "the perimeter of a rectangle with length 8 and width 3",
        "John has 10 apples and eats 3",
        "a store sells 50 items on Monday and 30 more on Tuesday",
    ]
    for f in fillers:
        for t in templates:
            tasks.append((t.format(f), "math"))
    return tasks

def gen_logic():
    tasks = [
        ("Five people are sitting in a row. Alice is next to Bob. Bob is not at the end. Who is in position 3?", "logical"),
        ("There are three boxes: red, blue, green. Each box has a different animal. The cat is not in the red box. The dog is next to the cat. Which animal is in the blue box?", "logical"),
        ("Four friends: Tom, Dick, Harry, and Sally each play a different sport. Tom does not play tennis. Sally plays soccer. Dick plays basketball. What sport does Harry play?", "logical"),
        ("If it is raining, then the ground is wet. The ground is not wet. What can you conclude?", "logical"),
        ("Three people: A, B, C are sitting in a row. C must be at one end. B is immediately to the left of A. What is the order?", "logical"),
        ("Every card with a vowel on one side has an even number on the other. Which cards must you flip to test this rule?", "logical"),
        ("All mammals are warm-blooded. Whales are mammals. What follows?", "logical"),
        ("A puzzle: a farmer needs to cross a river with a wolf, goat, and cabbage. How does he get all three across?", "logical"),
        ("Seating arrangement: six people sitting around a table. John is opposite Mary. Who is sitting next to John?", "logical"),
        ("Deduce: if all A are B and all B are C, then what?", "logical"),
        ("Neither Alice nor Bob can be first. Carol must be third. Who can be first?", "logical"),
        ("Exactly one of the following statements is true. Which one?", "logical"),
        ("Based on the clues, who owns the zebra?", "logical"),
    ]
    return tasks

def gen_sentiment():
    tasks = [
        ("Classify the sentiment of this review: The product is amazing and works perfectly!", "sentiment"),
        ("Sentiment analysis: I hate this movie, it was a complete waste of time", "sentiment"),
        ("Is this positive or negative? The food was decent but service was slow", "sentiment"),
        ("What is the sentiment? Absolutely fantastic experience, highly recommend!", "sentiment"),
        ("Analyze sentiment: The battery life is terrible and the screen is too dim", "sentiment"),
        ("Determine sentiment: It was okay, nothing special but not bad either", "sentiment"),
        ("The restaurant was wonderful, the staff were friendly", "sentiment"),
        ("This is the worst purchase I have ever made", "sentiment"),
        ("Sentiment classification: Boring and predictable plot", "sentiment"),
        ("Positive or negative? Not bad for the price", "sentiment"),
        ("What sentiment does this express? Thrilled with the results!", "sentiment"),
        ("sentiment of: The delivery was late and the package was damaged", "sentiment"),
    ]
    return tasks

def gen_ner():
    tasks = [
        ("Extract all named entities: Barack Obama was born in Hawaii", "ner"),
        ("Find all names, locations, dates: Apple Inc. was founded by Steve Jobs in Cupertino on April 1, 1976", "ner"),
        ("Identify all entities: Google's CEO Sundar Pichai announced the new Pixel phone", "ner"),
        ("NER: The Eiffel Tower is in Paris, France. It was built in 1889 by Gustave Eiffel", "ner"),
        ("Find people, places, and organizations: NASA's Artemis mission launched from Kennedy Space Center", "ner"),
        ("Extract entities: Elon Musk is the CEO of Tesla and SpaceX", "ner"),
        ("Named entity recognition: William Shakespeare wrote Hamlet in 1601", "ner"),
        ("Identify all named entities in: Microsoft acquired GitHub for $7.5 billion in 2018", "ner"),
        ("NER extraction: The Amazon rainforest spans across Brazil, Peru, and Colombia", "ner"),
        ("Find all proper nouns: John visited the British Museum in London last Tuesday", "ner"),
    ]
    return tasks

def gen_summarization():
    tasks = [
        ("Summarize the following text: The quick brown fox jumps over the lazy dog. The dog wakes up and chases the fox through the forest.", "summarization"),
        ("Write a summary of: Climate change is causing rising sea levels and extreme weather events around the world.", "summarization"),
        ("Summarize this: In recent years, artificial intelligence has transformed industries from healthcare to transportation.", "summarization"),
        ("Provide a brief summary: The novel tells the story of a young wizard who discovers his magical heritage.", "summarization"),
        ("Give a concise summary of this paragraph:", "summarization"),
        ("Summarize the key points from:", "summarization"),
        ("TL;DR:", "summarization"),
        ("In one sentence, summarize:", "summarization"),
        ("Can you summarize this article for me?", "summarization"),
        ("Provide a short summary of the following passage:", "summarization"),
        ("Write a one-paragraph summary:", "summarization"),
    ]
    return tasks

def gen_factual():
    tasks = [
        ("What is the capital of France?", "factual"),
        ("Who wrote Romeo and Juliet?", "factual"),
        ("What is the boiling point of water?", "factual"),
        ("What is the largest planet in our solar system?", "factual"),
        ("When was the Declaration of Independence signed?", "factual"),
        ("What is the speed of light?", "factual"),
        ("How many bones are in the human body?", "factual"),
        ("What is the chemical symbol for gold?", "factual"),
        ("Who discovered penicillin?", "factual"),
        ("What year did World War II end?", "factual"),
        ("What is the tallest mountain in the world?", "factual"),
        ("How many continents are there?", "factual"),
        ("What language is spoken in Brazil?", "factual"),
        ("What is the currency of Japan?", "factual"),
        ("Name the seven dwarfs from Snow White", "factual"),
        ("What does HTML stand for?", "factual"),
        ("How many bits are in a byte?", "factual"),
        ("What is the powerhouse of the cell?", "factual"),
        ("Explain the difference between a virus and a bacterium", "factual"),
        ("Explain the difference between AI and machine learning", "factual"),
        ("Explain how photosynthesis works", "factual"),
        ("Explain the process of evolution", "factual"),
        ("What is the difference between weather and climate?", "factual"),
        ("Compare reptiles and amphibians", "factual"),
        ("Describe the water cycle", "factual"),
        ("What is the theory of relativity?", "factual"),
        ("Explain how a car engine works", "factual"),
        ("Describe the structure of DNA", "factual"),
        ("Explain the meaning of this quote", "factual"),
        ("What is the definition of democracy?", "factual"),
        ("Explain the causes of World War I", "factual"),
        ("What does this word mean?", "factual"),
        ("Describe how a computer works", "factual"),
        ("Explain the concept of gravity", "factual"),
        ("Tell me about the history of the Internet", "factual"),
        ("What is the difference between TCP and UDP?", "factual"),
        ("Define what a black hole is", "factual"),
        ("Describe the respiratory system", "factual"),
    ]
    return tasks

def gen_extraction():
    tasks = [
        ("Extract the email address from: Contact us at support@example.com for help", "extraction"),
        ("Find all phone numbers in: Call me at 555-123-4567 or 555-987-6543", "extraction"),
        ("Extract the date from: The meeting is scheduled for March 15, 2024", "extraction"),
        ("Pull out the price from: The item costs $49.99 with free shipping", "extraction"),
        ("Extract the URL from: Visit our website at https://example.com for more info", "extraction"),
        ("Find the IP address: The server is at 192.168.1.1", "extraction"),
        ("Extract the product code: SKU-12345-XYZ is the item identifier", "extraction"),
        ("Pull the name and address from: John Doe, 123 Main St, Springfield, IL 62701", "extraction"),
        ("Extract all hashtags: Loving #sunset and #vacation in #hawaii", "extraction"),
        ("Find the time in: The store opens at 9:00 AM and closes at 9:00 PM", "extraction"),
        ("Extract the order number from: Your order #ORD-789456 has been shipped", "extraction"),
        ("Pull out all numbers: Room 42 on floor 7, building 3A", "extraction"),
    ]
    return tasks

# Collect all data
data = []
data.extend(gen_code_gen())
data.extend(gen_code_debug())
data.extend(gen_math())
data.extend(gen_logic())
data.extend(gen_sentiment())
data.extend(gen_ner())
data.extend(gen_summarization())
data.extend(gen_factual())
data.extend(gen_extraction())

# --- TF-IDF + Logistic Regression ---
random.shuffle(data)
texts = [d[0] for d in data]
labels = [d[1] for d in data]

# Build vocab (limit to top features)
tokenized = [tokenize(t) for t in texts]
all_words = [w for tokens in tokenized for w in tokens]
word_counts = Counter(all_words)
vocab_list = [w for w, c in word_counts.most_common(2000) if c >= 2]
vocab = {w: i for i, w in enumerate(vocab_list)}
n_features = len(vocab)

# Compute IDF
n_docs = len(tokenized)
df = Counter()
for tokens in tokenized:
    for w in set(tokens):
        if w in vocab:
            df[w] += 1

idf = {}
for w, idx in vocab.items():
    idf[w] = math.log((n_docs + 1) / (df[w] + 1)) + 1

# Build feature matrix (binary TF * IDF)
def transform(tokens):
    features = {}
    for w in set(tokens):
        if w in vocab:
            features[vocab[w]] = idf[w]
    return features

X = [transform(t) for t in tokenized]

# One-hot encode labels
label_set = sorted(set(labels))
label_to_idx = {l: i for i, l in enumerate(label_set)}
n_classes = len(label_set)
Y = [label_to_idx[l] for l in labels]

# Train Logistic Regression (OvR) with SGD
n_epochs = 50
lr = 0.1
l2 = 0.01

coef = [[0.0] * n_features for _ in range(n_classes)]
intercept = [0.0] * n_classes

for epoch in range(n_epochs):
    for xi, yi in zip(X, Y):
        # Compute scores
        scores = [intercept[c] + sum(coef[c][f] * v for f, v in xi.items()) for c in range(n_classes)]
        # Softmax
        max_s = max(scores)
        exp_s = [math.exp(s - max_s) for s in scores]
        sum_exp = sum(exp_s)
        probs = [e / sum_exp for e in exp_s]
        # Gradient step
        for c in range(n_classes):
            grad = probs[c] - (1.0 if c == yi else 0.0)
            intercept[c] -= lr * grad
            for f, v in xi.items():
                coef[c][f] -= lr * (grad * v + l2 * coef[c][f])

# --- Evaluate ---
correct = 0
for xi, yi in zip(X, Y):
    scores = [intercept[c] + sum(coef[c][f] * v for f, v in xi.items()) for c in range(n_classes)]
    pred = max(range(n_classes), key=lambda i: scores[i])
    if pred == yi:
        correct += 1
print(f"Training accuracy: {correct}/{len(Y)} = {100*correct/len(Y):.1f}%")

# --- Export ---
# Keep only non-zero feature columns to save space
active_features = set()
for c in range(n_classes):
    for f in range(n_features):
        if abs(coef[c][f]) > 1e-6:
            active_features.add(f)

active_list = sorted(active_features)
feature_map = {old: new for new, old in enumerate(active_list)}

vocab_out = {w: feature_map[idx] for w, idx in vocab.items() if idx in active_features}
idf_out = {w: idf[w] for w in vocab_out}
coef_out = [[coef[c][f] for f in active_list] for c in range(n_classes)]

model = {
    "vocab": vocab_out,
    "idf": idf_out,
    "coef": coef_out,
    "intercept": intercept,
    "classes": label_set,
}

with open("internal/classify/router_weights.json", "w") as f:
    json.dump(model, f, indent=2)

print(f"Exported weights to internal/classify/router_weights.json")
print(f"Vocab size: {len(vocab_out)}")
print(f"Classes: {label_set}")
print(f"Non-zero features per class: {[sum(1 for v in coef[c] if abs(v) > 1e-6) for c in range(n_classes)]}")

# --- Test inference on sample ---
def predict(text):
    tokens = tokenize(text)
    scores = list(intercept)
    for w in set(tokens):
        if w in vocab_out:
            f = vocab_out[w]
            v = idf_out[w]
            for c in range(n_classes):
                scores[c] += coef_out[c][f] * v
    return label_set[max(range(n_classes), key=lambda i: scores[i])]

print("\nSample predictions:")
samples = [
    "Write a function to sort a list",
    "Fix the bug in this code",
    "What is 15 plus 27?",
    "Five people are sitting in a row",
    "Sentiment analysis: I love this product",
    "Extract all named entities from this text",
    "Summarize the following article",
    "What is the capital of France?",
    "Extract the email address from this text",
]
for s in samples:
    print(f"  [{predict(s):18s}] {s}")
