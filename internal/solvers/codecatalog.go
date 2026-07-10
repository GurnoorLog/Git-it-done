package solvers

import (
	"strings"
)

type catalogEntry struct {
	keywords []string
	code     string
}

var codeCatalog = []catalogEntry{
	{
		keywords: []string{"reverse", "string"},
		code: `def reverse_string(s):
    return s[::-1]`,
	},
	{
		keywords: []string{"factorial"},
		code: `def factorial(n):
    if n < 0:
        raise ValueError("Factorial not defined for negative numbers")
    result = 1
    for i in range(2, n + 1):
        result *= i
    return result`,
	},
	{
		keywords: []string{"prime"},
		code: `def is_prime(n):
    if n < 2:
        return False
    if n < 4:
        return True
    if n % 2 == 0 or n % 3 == 0:
        return False
    i = 5
    while i * i <= n:
        if n % i == 0 or n % (i + 2) == 0:
            return False
        i += 6
    return True`,
	},
	{
		keywords: []string{"palindrome"},
		code: `def is_palindrome(s):
    cleaned = ''.join(c.lower() for c in s if c.isalnum())
    return cleaned == cleaned[::-1]`,
	},
	{
		keywords: []string{"fibonacci"},
		code: `def fibonacci(n):
    if n <= 0:
        return 0
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b`,
	},
	{
		keywords: []string{"anagram"},
		code: `def is_anagram(s1, s2):
    return sorted(s1.replace(" ", "").lower()) == sorted(s2.replace(" ", "").lower())`,
	},
	{
		keywords: []string{"vowel", "count"},
		code: `def count_vowels(s):
    vowels = set("aeiouAEIOU")
    return sum(1 for c in s if c in vowels)`,
	},
	{
		keywords: []string{"fizzbuzz"},
		code: `def fizzbuzz(n):
    result = []
    for i in range(1, n + 1):
        if i % 15 == 0:
            result.append("FizzBuzz")
        elif i % 3 == 0:
            result.append("Fizz")
        elif i % 5 == 0:
            result.append("Buzz")
        else:
            result.append(str(i))
    return result`,
	},
	{
		keywords: []string{"second", "largest"},
		code: `def second_largest(nums):
    if not nums or len(nums) < 2:
        return None
    first = second = float('-inf')
    for n in nums:
        if n > first:
            second = first
            first = n
        elif n > second and n != first:
            second = n
    return second if second != float('-inf') else None`,
	},
	{
		keywords: []string{"duplicate"},
		code: `def has_duplicate(nums):
    seen = set()
    for n in nums:
        if n in seen:
            return True
        seen.add(n)
    return False`,
	},
	{
		keywords: []string{"missing", "number"},
		code: `def find_missing(nums, n):
    total = n * (n + 1) // 2
    return total - sum(nums)`,
	},
	{
		keywords: []string{"two", "sum"},
		code: `def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        complement = target - n
        if complement in seen:
            return [seen[complement], i]
        seen[n] = i
    return []`,
	},
	{
		keywords: []string{"max"},
		code: `def find_max(nums):
    if not nums:
        return None
    max_val = nums[0]
    for n in nums[1:]:
        if n > max_val:
            max_val = n
    return max_val`,
	},
	{
		keywords: []string{"min"},
		code: `def find_min(nums):
    if not nums:
        return None
    min_val = nums[0]
    for n in nums[1:]:
        if n < min_val:
            min_val = n
    return min_val`,
	},
	{
		keywords: []string{"smallest"},
		code: `def find_min(nums):
    if not nums:
        return None
    min_val = nums[0]
    for n in nums[1:]:
        if n < min_val:
            min_val = n
    return min_val`,
	},
	{
		keywords: []string{"largest"},
		code: `def find_max(nums):
    if not nums:
        return None
    max_val = nums[0]
    for n in nums[1:]:
        if n > max_val:
            max_val = n
    return max_val`,
	},
	{
		keywords: []string{"sort"},
		code: `def sort_array(nums):
    return sorted(nums)`,
	},
	{
		keywords: []string{"frequency"},
		code: `def count_frequencies(nums):
    freq = {}
    for n in nums:
        freq[n] = freq.get(n, 0) + 1
    return freq`,
	},
	{
		keywords: []string{"binary search"},
		code: `def binary_search(nums, target):
    left, right = 0, len(nums) - 1
    while left <= right:
        mid = (left + right) // 2
        if nums[mid] == target:
            return mid
        elif nums[mid] < target:
            left = mid + 1
        else:
            right = mid - 1
    return -1`,
	},
	{
		keywords: []string{"gcd"},
		code: `def gcd(a, b):
    while b:
        a, b = b, a % b
    return abs(a)`,
	},
	{
		keywords: []string{"leap year"},
		code: `def is_leap_year(year):
    return year % 4 == 0 and (year % 100 != 0 or year % 400 == 0)`,
	},
	{
		keywords: []string{"celsius"},
		code: `def celsius_to_fahrenheit(c):
    return (c * 9/5) + 32

def fahrenheit_to_celsius(f):
    return (f - 32) * 5/9`,
	},
	{
		keywords: []string{"fahrenheit"},
		code: `def celsius_to_fahrenheit(c):
    return (c * 9/5) + 32

def fahrenheit_to_celsius(f):
    return (f - 32) * 5/9`,
	},
	{
		keywords: []string{"temperature"},
		code: `def celsius_to_fahrenheit(c):
    return (c * 9/5) + 32

def fahrenheit_to_celsius(f):
    return (f - 32) * 5/9`,
	},
	{
		keywords: []string{"even"},
		code: `def is_even(n):
    return n % 2 == 0

def is_odd(n):
    return n % 2 != 0`,
	},
	{
		keywords: []string{"odd"},
		code: `def is_even(n):
    return n % 2 == 0

def is_odd(n):
    return n % 2 != 0`,
	},
	{
		keywords: []string{"sum", "array"},
		code: `def sum_array(nums):
    total = 0
    for n in nums:
        total += n
    return total`,
	},
	{
		keywords: []string{"sum", "list"},
		code: `def sum_array(nums):
    total = 0
    for n in nums:
        total += n
    return total`,
	},
	{
		keywords: []string{"merge"},
		code: `def merge_sorted(arr1, arr2):
    result = []
    i = j = 0
    while i < len(arr1) and j < len(arr2):
        if arr1[i] < arr2[j]:
            result.append(arr1[i])
            i += 1
        else:
            result.append(arr2[j])
            j += 1
    result.extend(arr1[i:])
    result.extend(arr2[j:])
    return result`,
	},
	{
		keywords: []string{"capitalize"},
		code: `def capitalize_words(s):
    return ' '.join(word.capitalize() for word in s.split())`,
	},
}

// LookupCodeTemplate checks if a prompt matches a known pattern and returns
// the pre-written code. Returns empty string on no match.
// Matching is AND-based: ALL keywords must be present in the prompt.
func LookupCodeTemplate(prompt string) string {
	lower := strings.ToLower(prompt)
	for _, entry := range codeCatalog {
		allMatch := true
		for _, kw := range entry.keywords {
			if !strings.Contains(lower, kw) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return entry.code
		}
	}
	return ""
}
