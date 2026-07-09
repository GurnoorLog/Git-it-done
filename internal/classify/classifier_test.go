package classify

import (
	"testing"
)

type testCase struct {
	prompt   string
	expected Category
}

var testCases = []testCase{
	// ── Math (10 examples) ──────────────────────────────────────────────────
	{
		prompt:   "A store sells a jacket for $120. If there is a 25% discount, what is the final price?",
		expected: CategoryMath,
	},
	{
		prompt:   "A company had revenue of $500,000 last year and expects 15% growth this year. What is the projected revenue?",
		expected: CategoryMath,
	},
	{
		prompt:   "If 3 workers can complete a job in 12 days, how many days will 9 workers take to finish the same job?",
		expected: CategoryMath,
	},
	{
		prompt:   "Calculate the total cost if you buy 4 items at $7.50 each and 2 items at $12 each, with 8% tax applied.",
		expected: CategoryMath,
	},
	{
		prompt:   "A car travels 240 miles in 4 hours. What is the average speed in miles per hour?",
		expected: CategoryMath,
	},
	{
		prompt:   "If the population of a city is 2 million and it grows at 3% per year, what will it be in 5 years?",
		expected: CategoryMath,
	},
	{
		prompt:   "A product costs $80 to manufacture and is sold for $120. What is the profit margin as a percent?",
		expected: CategoryMath,
	},
	{
		prompt:   "John earns $55,000 per year. If he gets a 10% raise, what will his new salary be?",
		expected: CategoryMath,
	},
	{
		prompt:   "How many total students are in the school if there are 30 students per class and 24 classes?",
		expected: CategoryMath,
	},
	{
		prompt:   "A loan of $10,000 has an annual interest rate of 6%. How much interest is owed after 2 years?",
		expected: CategoryMath,
	},

	// ── Sentiment (10 examples) ─────────────────────────────────────────────
	{
		prompt:   "Classify the sentiment of the following review: 'The food was terrible and the service was rude.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "What is the overall sentiment of this tweet: 'I absolutely love the new update! Best app ever!'",
		expected: CategorySentiment,
	},
	{
		prompt:   "Determine whether the tone of the following text is positive, negative, or neutral.",
		expected: CategorySentiment,
	},
	{
		prompt:   "Analyze the sentiment expressed in this customer feedback and label it as positive, negative, or neutral.",
		expected: CategorySentiment,
	},
	{
		prompt:   "Is the following product review expressing a positive or negative emotion? 'This is the worst purchase I have ever made.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "What emotion is conveyed in this sentence: 'I was disappointed by the lack of communication from the team.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "Classify the opinion expressed: 'The movie was mediocre, not great but not terrible either.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "Is the attitude in the following text positive, negative, or neutral? 'The hotel was clean and the staff was helpful.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "Detect the sentiment in this feedback: 'Shipping was slow but the product quality exceeded my expectations.'",
		expected: CategorySentiment,
	},
	{
		prompt:   "What is the mood of this passage? 'She walked into the room with a heavy heart, dreading what was about to come.'",
		expected: CategorySentiment,
	},

	// ── Summarization (10 examples) ─────────────────────────────────────────
	{
		prompt:   "Summarize the following article in 3 sentences: The global economy has faced significant challenges over the past decade. Rising inflation, supply chain disruptions caused by the pandemic, and geopolitical tensions have all contributed to economic instability. Central banks around the world have responded with a combination of interest rate adjustments and quantitative easing measures. Despite these interventions, growth projections remain uncertain, with many economists predicting a period of prolonged slow growth or even recession in several major economies. Consumer confidence has dropped significantly, impacting spending patterns and business investment.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Condense the following text into its key points: Artificial intelligence is transforming virtually every industry from healthcare to finance. Machine learning models can now diagnose diseases with accuracy comparable to expert physicians, detect fraud in financial transactions in real time, and power autonomous vehicles that navigate complex environments. The pace of AI development has accelerated dramatically, with new breakthroughs announced almost weekly. However, this rapid advancement brings significant ethical challenges including questions about bias, privacy, transparency, and the displacement of workers. Policymakers worldwide are scrambling to develop regulatory frameworks that can keep pace with the technology.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Please write a brief summary of the passage below in 2 sentences: Climate change represents one of the greatest threats facing humanity. The scientific consensus is unambiguous: human activities, particularly the burning of fossil fuels, are driving global temperatures to rise at an unprecedented rate. This warming is already causing more frequent and intense extreme weather events, rising sea levels that threaten coastal communities, and disruptions to agriculture and water supplies. Developing nations, which have contributed least to greenhouse gas emissions, often face the most severe impacts while having the fewest resources to adapt.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Give me the TL;DR of this article: Remote work has fundamentally reshaped the modern workplace since the pandemic forced companies worldwide to adopt distributed working models almost overnight. Studies show that productivity has largely been maintained or even improved for knowledge workers, while employees report higher satisfaction due to eliminated commutes and greater flexibility. However, challenges around collaboration, mentorship of junior employees, and maintaining company culture have emerged as significant concerns. Many companies are now adopting hybrid models that attempt to balance the benefits of both in-office and remote arrangements.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Shorten the following passage to its essential meaning: The human immune system is a remarkably complex network of cells, tissues, and organs that work together to defend the body against harmful pathogens such as bacteria, viruses, and parasites. When a foreign substance enters the body, specialized white blood cells called lymphocytes recognize it as foreign and mount an immune response. B cells produce antibodies that neutralize specific pathogens, while T cells coordinate the immune response and directly kill infected cells. The immune system also has a memory component that allows it to respond more quickly and effectively to pathogens it has encountered before, which is the basis for how vaccines work.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Extract the main points from this long document: The history of the internet dates back to the 1960s when the United States Department of Defense funded research into packet switching networks. ARPANET, the precursor to the modern internet, was established in 1969 and initially connected just four university computers. Over the following decades, the network expanded gradually, primarily within academic and research institutions. The invention of the World Wide Web by Tim Berners-Lee in 1989 transformed the internet from a specialized research tool into a global communication platform. The commercialization of the internet in the 1990s led to explosive growth, the dot-com boom and bust, and ultimately the emergence of the digital economy we know today.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Summarize in one paragraph what the text is about: Quantum computing represents a fundamentally different approach to computation that harnesses quantum mechanical phenomena such as superposition and entanglement. Unlike classical computers that process information in bits representing either 0 or 1, quantum computers use qubits that can exist in multiple states simultaneously. This property enables quantum computers to explore many possible solutions to a problem at once, making them potentially far more powerful than classical computers for specific types of problems such as cryptography, drug discovery, and optimization. However, quantum computers are extremely difficult to build and maintain, requiring temperatures near absolute zero and being highly sensitive to environmental interference.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Write a brief abstract for the following text: Blockchain technology, the foundation of cryptocurrencies like Bitcoin, is a distributed ledger system that records transactions across multiple computers in a way that makes them resistant to modification. Each block in the chain contains a cryptographic hash of the previous block, a timestamp, and transaction data. This structure makes it virtually impossible to alter historical records without redoing all subsequent blocks and achieving consensus from the network. Beyond cryptocurrencies, blockchain is being explored for applications in supply chain management, digital identity verification, smart contracts, and decentralized finance.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Please give me a concise overview of this material: The Renaissance was a cultural movement that began in Italy in the 14th century and spread throughout Europe over the following two centuries. It marked a transition from the medieval period to the modern world and was characterized by a renewed interest in classical Greek and Roman culture, humanism, artistic innovation, and scientific inquiry. Prominent figures of the Renaissance include Leonardo da Vinci, Michelangelo, Raphael, Galileo Galilei, and Nicolaus Copernicus. The period saw revolutionary developments in art, architecture, literature, philosophy, music, science, politics, and religion.",
		expected: CategorySummarization,
	},
	{
		prompt:   "Summarize the key findings described below: A recent study published in the journal Nature found that global bee populations have declined by approximately 30% over the past decade. Researchers attribute this decline to a combination of factors including pesticide use, habitat destruction, parasites such as the Varroa mite, and climate change. The loss of pollinators poses a serious threat to agricultural productivity, as roughly one third of the world's food supply depends on pollination by bees and other insects. The study recommends a combination of reduced pesticide use, habitat restoration, and climate action as necessary steps to reverse the decline.",
		expected: CategorySummarization,
	},

	// ── NER (10 examples) ───────────────────────────────────────────────────
	{
		prompt:   "Extract all named entities from the following text and return them as JSON with fields for persons, organizations, and locations: 'Apple Inc. was founded by Steve Jobs in Cupertino, California in 1976.'",
		expected: CategoryNER,
	},
	{
		prompt:   "Identify all people, companies, and places mentioned in this paragraph.",
		expected: CategoryNER,
	},
	{
		prompt:   "List all organization names and dates mentioned in the following news article.",
		expected: CategoryNER,
	},
	{
		prompt:   "Find all named entities in this text. Return a structured JSON with entity type and entity value.",
		expected: CategoryNER,
	},
	{
		prompt:   "Extract the names of all persons and their associated organizations from the passage below.",
		expected: CategoryNER,
	},
	{
		prompt:   "What entities of type PERSON, ORGANIZATION, and LOCATION can you identify in this document?",
		expected: CategoryNER,
	},
	{
		prompt:   "Perform named entity recognition on the following text and categorize entities as person, place, or organization.",
		expected: CategoryNER,
	},
	{
		prompt:   "Identify all product names, company names, and locations in the following customer review.",
		expected: CategoryNER,
	},
	{
		prompt:   "Extract all dates, people, and events from this historical text and output them as a list.",
		expected: CategoryNER,
	},
	{
		prompt:   "Find every mention of a company name or person name in the text and return it in a JSON array.",
		expected: CategoryNER,
	},

	// ── Code Debugging (10 examples) ────────────────────────────────────────
	{
		prompt:   "This Python function has a bug, can you fix it?\n```python\ndef divide(a, b):\n    return a / b\n\nresult = divide(10, 0)\nprint(result)\n```",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "My code doesn't work. Here is the error: IndexError: list index out of range.\n```python\nmy_list = [1, 2, 3]\nprint(my_list[5])\n```",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "Why does this JavaScript function return undefined instead of the expected value?\n```javascript\nfunction greet(name) {\n    let message = 'Hello, ' + name;\n}\ngreet('Alice');\n```",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "Debug this Go code that panics at runtime:\n```go\npackage main\nfunc main() {\n    var s []int\n    _ = s[0]\n}\n```",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "There's an issue with my sorting algorithm. It doesn't work correctly for negative numbers.\n```python\ndef bubble_sort(arr):\n    n = len(arr)\n    for i in range(n):\n        for j in range(n-i):\n            if arr[j] > arr[j+1]:\n                arr[j], arr[j+1] = arr[j+1], arr[j]\n    return arr\n```\nHelp me fix it.",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "This SQL query is broken and I can't figure out why:\n```sql\nSELECT name FROM users WHERE id = '5\n```\nWhat's wrong?",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "My React component crashes with a TypeError. Here's the code:\n```jsx\nfunction UserCard({ user }) {\n  return <div>{user.name.toUpperCase()}</div>;\n}\n```\nfix it",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "Find the bug in this recursive function:\n```python\ndef factorial(n):\n    if n == 1:\n        return 1\n    return n * factorial(n)\n```\nIt causes infinite recursion.",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "This code compiles but gives the wrong output. Can you identify the error?\n```go\nfunc sum(nums []int) int {\n    total := 0\n    for i := 0; i < len(nums); i++ {\n        total += nums[i+1]\n    }\n    return total\n}\n```",
		expected: CategoryCodeDebugging,
	},
	{
		prompt:   "My program throws an exception: NullPointerException at line 3. Help me debug:\n```java\npublic class Main {\n    static String name = null;\n    public static void main(String[] args) {\n        System.out.println(name.length());\n    }\n}\n```",
		expected: CategoryCodeDebugging,
	},

	// ── Code Generation (10 examples) ───────────────────────────────────────
	{
		prompt:   "Write a function in Python that takes a list of integers and returns the second largest value.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Implement a binary search algorithm in Go that searches for a target value in a sorted slice.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Create a function that validates an email address using a regular expression in JavaScript.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Write a class in Python that implements a stack data structure with push, pop, and peek methods.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Generate code for a REST API endpoint in Go that accepts POST requests and returns a JSON response.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Implement a function that reverses a linked list in Java.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Write a Python script that reads a CSV file and computes the mean and standard deviation of a numeric column.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Create a module that implements the LRU cache eviction policy with get and put operations.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Write a function to check if a given string is a palindrome, ignoring spaces and punctuation.",
		expected: CategoryCodeGeneration,
	},
	{
		prompt:   "Implement a priority queue in Go using a min-heap.",
		expected: CategoryCodeGeneration,
	},

	// ── Logical/Deductive (10 examples) ─────────────────────────────────────
	{
		prompt:   "Five people — Alice, Bob, Carol, Dave, and Eve — are seated in a row. Alice is not next to Bob. Carol is immediately to the left of Dave. Eve is either first or last. Bob is not in position 3. Who is in position 2?",
		expected: CategoryLogical,
	},
	{
		prompt:   "A, B, and C are suspects. Exactly one of them is guilty. A says: 'I am innocent.' B says: 'A is telling the truth.' C says: 'B is lying.' If exactly one person is lying, who is guilty?",
		expected: CategoryLogical,
	},
	{
		prompt:   "If it is raining, then the ground is wet. The ground is not wet. Therefore, is it raining?",
		expected: CategoryLogical,
	},
	{
		prompt:   "Three boxes contain either apples, oranges, or both. All labels are wrong. Box 1 says 'apples', Box 2 says 'oranges', Box 3 says 'apples and oranges'. You can only pick one fruit from one box. Which box do you pick from to determine all contents?",
		expected: CategoryLogical,
	},
	{
		prompt:   "In a tournament, every player must play every other player exactly once. If there are 6 players, and A beats B, B beats C, C beats A. D beats A, B, and C. E beats A but loses to B, C, D. F loses to everyone. Who finishes in position 3?",
		expected: CategoryLogical,
	},
	{
		prompt:   "A farmer must cross a river with a wolf, a goat, and a cabbage. The boat can hold him and one item. If left alone, the wolf eats the goat, and the goat eats the cabbage. How does he get all three across?",
		expected: CategoryLogical,
	},
	{
		prompt:   "Exactly one of the following statements is true: (1) Statement 2 is true. (2) Statement 1 is false. Which statement is true?",
		expected: CategoryLogical,
	},
	{
		prompt:   "Five houses in a row are each painted a different color. The English man lives in the red house. The Swede has dogs. The Dane drinks tea. The green house is immediately left of the white house. Who drinks water?",
		expected: CategoryLogical,
	},
	{
		prompt:   "All mammals are warm-blooded. Dolphins are mammals. Whales are not fish. Bats are not birds but are mammals. Which of these conclusions must be true: (a) Dolphins are warm-blooded, (b) Whales are warm-blooded, (c) Bats can fly?",
		expected: CategoryLogical,
	},
	{
		prompt:   "There are 4 cards face-down showing A, B, 2, 3. Every card with a vowel on one side must have an even number on the other side. Which cards must you flip to verify this rule?",
		expected: CategoryLogical,
	},

	// ── Factual Knowledge (10 examples) ─────────────────────────────────────
	{
		prompt:   "What is the capital city of Australia?",
		expected: CategoryFactual,
	},
	{
		prompt:   "How does photosynthesis work?",
		expected: CategoryFactual,
	},
	{
		prompt:   "Who invented the telephone?",
		expected: CategoryFactual,
	},
	{
		prompt:   "Explain the difference between a virus and a bacterium.",
		expected: CategoryFactual,
	},
	{
		prompt:   "What year did World War II end?",
		expected: CategoryFactual,
	},
	{
		prompt:   "What is the speed of light in a vacuum?",
		expected: CategoryFactual,
	},
	{
		prompt:   "How does the human digestive system work?",
		expected: CategoryFactual,
	},
	{
		prompt:   "What is the largest planet in the solar system?",
		expected: CategoryFactual,
	},
	{
		prompt:   "Explain what DNA replication is.",
		expected: CategoryFactual,
	},
	{
		prompt:   "What causes the northern lights (aurora borealis)?",
		expected: CategoryFactual,
	},
}

func TestClassifier(t *testing.T) {
	correct := 0
	total := len(testCases)

	for _, tc := range testCases {
		got := Classify(tc.prompt)
		if got == tc.expected {
			correct++
		} else {
			t.Errorf("FAIL prompt=%q\n  expected=%s, got=%s",
				truncate(tc.prompt, 80), tc.expected, got)
		}
	}

	accuracy := float64(correct) / float64(total) * 100
	t.Logf("Classifier accuracy: %d/%d = %.1f%%", correct, total, accuracy)

	const minAccuracy = 95.0
	if accuracy < minAccuracy {
		t.Errorf("accuracy %.1f%% is below required %.1f%%", accuracy, minAccuracy)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
